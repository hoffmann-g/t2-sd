package results

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// FaultRun is one repetition of E5 or E6: a run with instants marked by the
// script, and the timeline of each second relative to the first of them
// (the partition or the kill).
type FaultRun struct {
	Experiment string // "e5" or "e6"
	System     string
	Variant    string // E6: "leader" or "follower"; E5: ""
	Rep        string
	Dir        string
	Events     []Event
	FaultMs    int64  // instant of the partition or the kill
	RecoverMs  int64  // instant of the heal or the restart
	Victim     string // E6: node killed
	Leader     string // coordinator before the fault (leader-before.txt)

	// Timeline per second, every node and both operations.
	Timeline []Second

	// E6: time from the kill to the completion of the first PUT that was
	// sent after the kill and succeeded, in seconds. -1 if none. (The
	// samples hold every PUT, so this does not depend on -samples-all.)
	FailoverS float64
	// E6: the first node to answer a PUT after the kill.
	FirstWriter string

	// Requests in the phases of the run: before the fault (after the
	// warm-up), during it, and after the recovery.
	Before, During, After Phase

	// E5: result of the convergence check (check.json), if present.
	Check *Check
}

// Phase counts the requests completed in a stretch of the run, per node.
type Phase struct {
	Seconds int
	OK      int
	Errors  int
	PerNode map[string]*PhaseNode
}

// PhaseNode counts the requests one node answered in a phase.
type PhaseNode struct {
	GetOK, GetErr, PutOK, PutErr int
	// Unavailable is the number of seconds in which the node answered no
	// request at all, with errors or not.
	Unavailable int
}

// Check is check.json of E5.
type Check struct {
	Keys      int            `json:"keys"`
	Divergent int            `json:"divergent_keys"`
	Lost      map[string]int `json:"lost_writes_per_node"`
	Unknown   map[string]int `json:"unknown_values_per_node"`
	InFlight  map[string]int `json:"in_flight_values_per_node"`
	Missing   map[string]int `json:"missing_keys_per_node"`
	Errors    map[string]int `json:"read_errors_per_node"`
}

// LoadFaultRuns reads results/<exp>/<system>[/<variant>]/rep*/.
func LoadFaultRuns(resultsDir, exp string) ([]FaultRun, error) {
	systems, err := subdirs(filepath.Join(resultsDir, exp))
	if err != nil {
		return nil, err
	}
	var runs []FaultRun
	for _, system := range systems {
		base := filepath.Join(resultsDir, exp, system)
		variants := []string{""}
		if exp == "e6" {
			variants = []string{"leader", "follower"}
		}
		for _, variant := range variants {
			dirs, err := repDirs(filepath.Join(base, variant), "summary.json")
			if err != nil {
				return nil, err
			}
			for _, d := range dirs {
				r, err := loadFaultRun(exp, system, variant, d)
				if err != nil {
					return nil, err
				}
				runs = append(runs, r)
			}
		}
	}
	return runs, nil
}

func loadFaultRun(exp, system, variant, dir string) (FaultRun, error) {
	r := FaultRun{Experiment: exp, System: system, Variant: variant, Rep: filepath.Base(dir), Dir: dir, FailoverS: -1}
	var err error
	if r.Events, err = LoadEvents(dir); err != nil {
		return r, err
	}
	if r.Timeline, err = LoadTimeline(dir); err != nil {
		return r, err
	}
	run, err := LoadRun(dir)
	if err != nil {
		return r, err
	}
	if b, err := os.ReadFile(filepath.Join(dir, "leader-before.txt")); err == nil {
		r.Leader = strings.TrimSpace(string(b))
	}
	switch exp {
	case "e5":
		r.FaultMs, _, _ = Find(r.Events, "partition")
		r.RecoverMs, _, _ = Find(r.Events, "heal")
		if b, err := os.ReadFile(filepath.Join(dir, "check.json")); err == nil {
			r.Check = &Check{}
			if err := json.Unmarshal(b, r.Check); err != nil {
				return r, fmt.Errorf("%s: %w", dir, err)
			}
		}
	case "e6":
		var label string
		r.FaultMs, label, _ = Find(r.Events, "kill")
		r.Victim = strings.TrimPrefix(label, "kill ")
		r.RecoverMs, _, _ = Find(r.Events, "start")
		if err := r.failover(); err != nil {
			return r, err
		}
	}
	if r.FaultMs == 0 || r.RecoverMs == 0 {
		return r, fmt.Errorf("%s: events.csv lacks the fault or the recovery", dir)
	}
	startMs := run.StartUnixNs / 1e6
	warmEnd := startMs + int64(run.WarmupS*1000)
	r.Before = phase(r.Timeline, warmEnd, r.FaultMs)
	r.During = phase(r.Timeline, r.FaultMs, r.RecoverMs)
	r.After = phase(r.Timeline, r.RecoverMs, startMs+int64(run.DurationS*1000))
	return r, nil
}

// phase sums the seconds of the timeline whose start lies in [from, to).
func phase(tl []Second, fromMs, toMs int64) Phase {
	p := Phase{PerNode: map[string]*PhaseNode{}}
	seen := map[int]bool{}
	answered := map[string]map[int]bool{}
	for _, s := range tl {
		ms := s.UnixS * 1000
		if ms < fromMs || ms >= toMs {
			continue
		}
		if s.Node == "all" {
			if !seen[s.Sec] {
				seen[s.Sec] = true
				p.Seconds++
			}
			p.OK += s.OK
			p.Errors += s.Errors()
			continue
		}
		n := p.PerNode[s.Node]
		if n == nil {
			n = &PhaseNode{}
			p.PerNode[s.Node] = n
			answered[s.Node] = map[int]bool{}
		}
		if s.Op == "GET" {
			n.GetOK, n.GetErr = n.GetOK+s.OK, n.GetErr+s.Errors()
		} else {
			n.PutOK, n.PutErr = n.PutOK+s.OK, n.PutErr+s.Errors()
		}
		if s.OK+s.Errors() > 0 {
			answered[s.Node][s.Sec] = true
		}
	}
	for node, n := range p.PerNode {
		n.Unavailable = p.Seconds - len(answered[node])
	}
	return p
}

// failover scans the samples for the first successful PUT sent after the
// kill.
func (r *FaultRun) failover() error {
	if r.FaultMs == 0 {
		return nil
	}
	killNs := r.FaultMs * 1e6
	firstPut := int64(math.MaxInt64)
	err := EachSample(r.Dir, func(s SampleRow) {
		if s.Op == "PUT" && s.Status == "ok" && s.StartNs >= killNs && s.EndNs() < firstPut {
			firstPut, r.FirstWriter = s.EndNs(), s.Node
		}
	})
	if err != nil {
		return err
	}
	if firstPut != math.MaxInt64 {
		r.FailoverS = float64(firstPut-killNs) / 1e9
	}
	return nil
}

// FaultTable summarizes the fault runs: one row per run, with the requests
// of each phase per node.
func FaultTable(runs []FaultRun) Table {
	t := Table{Head: []string{"system", "variant", "rep", "coordinator", "fault", "phase", "s", "node", "GET ok", "GET err", "PUT ok", "PUT err", "silent s"}}
	for _, r := range runs {
		fault := "partition of n1"
		if r.Experiment == "e6" {
			fault = "kill " + r.Victim
		}
		for _, ph := range []struct {
			name string
			p    Phase
		}{{"before", r.Before}, {"during", r.During}, {"after", r.After}} {
			for _, node := range []string{"n1", "n2", "n3"} {
				n := ph.p.PerNode[node]
				if n == nil {
					n = &PhaseNode{Unavailable: ph.p.Seconds}
				}
				t.Add(r.System, r.Variant, r.Rep, r.Leader, fault, ph.name, fmt.Sprint(ph.p.Seconds), node,
					fmt.Sprint(n.GetOK), fmt.Sprint(n.GetErr), fmt.Sprint(n.PutOK), fmt.Sprint(n.PutErr), fmt.Sprint(n.Unavailable))
			}
		}
	}
	return t
}

// FailoverTable summarizes E6: one row per system and variant.
func FailoverTable(runs []FaultRun) Table {
	type key struct{ system, variant string }
	groups := map[key][]FaultRun{}
	var order []key
	for _, r := range runs {
		k := key{r.System, r.Variant}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], r)
	}
	t := Table{Head: []string{"system", "variant", "reps", "victims", "write failover s", "first writer after", "errors during the failure", "req/s during", "req/s before", "errors after restart", "req/s after"}}
	for _, k := range order {
		g := groups[k]
		var fo, errDuring, thrDuring, thrBefore, errAfter, thrAfter []float64
		victims := map[string]bool{}
		writers := map[string]bool{}
		for _, r := range g {
			victims[r.Victim] = true
			if r.FailoverS >= 0 {
				fo = append(fo, r.FailoverS)
				writers[r.FirstWriter] = true
			}
			errDuring = append(errDuring, float64(r.During.Errors))
			errAfter = append(errAfter, float64(r.After.Errors))
			thrDuring = append(thrDuring, rate(r.During))
			thrBefore = append(thrBefore, rate(r.Before))
			thrAfter = append(thrAfter, rate(r.After))
		}
		names := make([]string, 0, len(victims))
		for v := range victims {
			names = append(names, v)
		}
		fos := "none"
		if len(fo) > 0 {
			fos = rng(newStat(fo), f3)
		}
		var first []string
		for w := range writers {
			first = append(first, w)
		}
		t.Add(k.system, k.variant, fmt.Sprint(len(g)), strings.Join(names, " "), fos, strings.Join(first, " "),
			rng(newStat(errDuring), f0), rng(newStat(thrDuring), f0), rng(newStat(thrBefore), f0),
			rng(newStat(errAfter), f0), rng(newStat(thrAfter), f0))
	}
	return t
}

func rate(p Phase) float64 {
	if p.Seconds == 0 {
		return 0
	}
	return float64(p.OK) / float64(p.Seconds)
}

// CheckTable summarizes the convergence checks of E5.
func CheckTable(runs []FaultRun) Table {
	t := Table{Head: []string{"system", "rep", "coordinator", "divergent keys", "lost writes", "unknown values", "in flight at the end", "missing keys"}}
	for _, r := range runs {
		if r.Check == nil {
			t.Add(r.System, r.Rep, r.Leader, "no check.json", "", "", "", "")
			continue
		}
		per := func(m map[string]int) string {
			return fmt.Sprintf("n1 %d, n2 %d, n3 %d", m["n1"], m["n2"], m["n3"])
		}
		t.Add(r.System, r.Rep, r.Leader, fmt.Sprint(r.Check.Divergent), per(r.Check.Lost), per(r.Check.Unknown), per(r.Check.InFlight), per(r.Check.Missing))
	}
	return t
}

// WriteAlignedTimeline writes one CSV with every second of every run,
// with the time relative to the fault, for the plots of E5 and E6.
func WriteAlignedTimeline(path string, runs []FaultRun) error {
	t := Table{Head: []string{"system", "variant", "rep", "t_rel_s", "phase", "node", "op", "ok", "timeouts", "failed", "p50_ms", "p99_ms"}}
	for _, r := range runs {
		for _, s := range r.Timeline {
			ms := s.UnixS * 1000
			ph := "before"
			switch {
			case ms >= r.RecoverMs:
				ph = "after"
			case ms >= r.FaultMs:
				ph = "during"
			}
			t.Add(r.System, r.Variant, r.Rep, fmt.Sprintf("%.0f", float64(ms-r.FaultMs)/1000), ph, s.Node, s.Op,
				fmt.Sprint(s.OK), fmt.Sprint(s.Timeouts), fmt.Sprint(s.Failed), f3(s.P50Ms), f3(s.P99Ms))
		}
	}
	return t.WriteCSV(path)
}
