package results

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// staleSummary is the summary.json of the stale coordinator.
type staleSummary struct {
	System    string  `json:"system"`
	Read      string  `json:"read"`
	WriteNode string  `json:"write_node"`
	ReadNode  string  `json:"read_node"`
	Pairs     int     `json:"pairs"`
	DurationS float64 `json:"duration_s"`
	PutOK     int     `json:"put_ok"`
	PutFailed int     `json:"put_failed"`
	GetOK     int     `json:"get_ok"`
	GetFailed int     `json:"get_failed"`
	Stale     int     `json:"stale"`
	StaleRate float64 `json:"stale_rate"`
	MaxLag    int     `json:"max_lag"`
	Missing   int     `json:"missing"`
	PutLat    *struct {
		P50 float64 `json:"p50"`
		P99 float64 `json:"p99"`
	} `json:"put_latency_ms"`
	GetLat *struct {
		P50 float64 `json:"p50"`
		P99 float64 `json:"p99"`
	} `json:"get_latency_ms"`
}

// StaleResult is E3 for one system and condition over the repetitions.
type StaleResult struct {
	System    string
	Condition string // "normal" or "delay"
	Read      string
	Nodes     string // "PUT n1, GET n3"
	Reps      int
	Pairs     int
	StaleRate Stat // fraction of successful GETs that were stale
	Stale     int  // stale reads summed over the repetitions
	Gets      int  // successful GETs summed over the repetitions
	MaxLag    int
	PutErrors int
	GetErrors int
	PutP50    Stat
	PutP99    Stat
	GetP50    Stat
	GetP99    Stat
	RTT       string
}

// LoadE3 reads results/e3/<system>/<condition>/rep*/summary.json.
func LoadE3(resultsDir string) ([]StaleResult, error) {
	systems, err := subdirs(filepath.Join(resultsDir, "e3"))
	if err != nil {
		return nil, err
	}
	var out []StaleResult
	for _, system := range systems {
		for _, cond := range []string{"normal", "delay"} {
			// A repetition is split among coordinators: rep<r>/c<k>/summary.json.
			// A repetition with a single coordinator (rep<r>/summary.json) is
			// also accepted.
			repDir := filepath.Join(resultsDir, "e3", system, cond)
			reps, err := repDirs(repDir, "done")
			if err != nil {
				return nil, err
			}
			seen := map[string]bool{}
			for _, d := range reps {
				seen[d] = true
			}
			if single, _ := repDirs(repDir, "summary.json"); len(single) > 0 {
				for _, d := range single {
					if !seen[d] {
						reps = append(reps, d)
					}
				}
			}
			if len(reps) == 0 {
				continue
			}
			r := StaleResult{System: system, Condition: cond, Reps: len(reps)}
			var rate, p50, p99, g50, g99 []float64
			for _, rep := range reps {
				files, _ := filepath.Glob(filepath.Join(rep, "c*", "summary.json"))
				if _, err := os.Stat(filepath.Join(rep, "summary.json")); err == nil {
					files = append(files, filepath.Join(rep, "summary.json"))
				}
				var repStale, repGets, repPairs int
				for _, f := range files {
					b, err := os.ReadFile(f)
					if err != nil {
						return nil, err
					}
					var s staleSummary
					if err := json.Unmarshal(b, &s); err != nil {
						return nil, fmt.Errorf("%s: %w", f, err)
					}
					r.Read = s.Read
					r.Nodes = fmt.Sprintf("PUT %s, GET %s", s.WriteNode, s.ReadNode)
					repPairs += s.Pairs
					repStale += s.Stale
					repGets += s.GetOK
					r.MaxLag = max(r.MaxLag, s.MaxLag)
					r.PutErrors += s.PutFailed
					r.GetErrors += s.GetFailed
					if s.PutLat != nil {
						p50, p99 = append(p50, s.PutLat.P50), append(p99, s.PutLat.P99)
					}
					if s.GetLat != nil {
						g50, g99 = append(g50, s.GetLat.P50), append(g99, s.GetLat.P99)
					}
				}
				r.Pairs = repPairs
				r.Stale += repStale
				r.Gets += repGets
				if repGets > 0 {
					rate = append(rate, float64(repStale)/float64(repGets))
				}
				if rtt, err := os.ReadFile(filepath.Join(rep, "rtt.txt")); err == nil {
					r.RTT = lastLine(string(rtt))
				}
			}
			r.StaleRate = newStat(rate)
			r.PutP50, r.PutP99, r.GetP50, r.GetP99 = newStat(p50), newStat(p99), newStat(g50), newStat(g99)
			out = append(out, r)
		}
	}
	return out, nil
}

// StaleTable builds the stale-read table: one row per system and condition.
func StaleTable(list []StaleResult) Table {
	t := Table{Head: []string{"system", "condition", "read", "nodes", "reps", "pairs", "stale rate", "stale / GETs", "max lag", "PUT errors", "GET errors", "PUT p50 ms", "PUT p99 ms", "GET p50 ms", "GET p99 ms"}}
	for _, r := range list {
		t.Add(r.System, r.Condition, r.Read, r.Nodes, fmt.Sprint(r.Reps), fmt.Sprint(r.Pairs),
			rng(r.StaleRate, func(v float64) string { return fmt.Sprintf("%.2f%%", 100*v) }),
			fmt.Sprintf("%d / %d", r.Stale, r.Gets), fmt.Sprint(r.MaxLag),
			fmt.Sprint(r.PutErrors), fmt.Sprint(r.GetErrors),
			rng(r.PutP50, f3), rng(r.PutP99, f3), rng(r.GetP50, f3), rng(r.GetP99, f3))
	}
	return t
}
