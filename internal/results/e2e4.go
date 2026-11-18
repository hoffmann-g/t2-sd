package results

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Baseline is E2 or E4 for one system: the latency percentiles and the
// throughput over the repetitions.
type Baseline struct {
	Experiment string // "e2" or "e4"
	System     string
	Threads    int
	Reps       int
	Throughput Stat
	Errors     int
	Get, Put   OpStat
	GenUtil    float64
	RTT        string // E4: the RTT line of rtt.txt of the first repetition
}

// OpStat is the latency of one operation over the repetitions, in ms.
type OpStat struct {
	Throughput    Stat
	P50, P95, P99 Stat
}

// LoadBaselines reads results/<exp>/<system>/rep*/summary.json.
func LoadBaselines(resultsDir, exp string) ([]Baseline, error) {
	systems, err := subdirs(filepath.Join(resultsDir, exp))
	if err != nil {
		return nil, err
	}
	var out []Baseline
	for _, system := range systems {
		dirs, err := repDirs(filepath.Join(resultsDir, exp, system), "summary.json")
		if err != nil {
			return nil, err
		}
		if len(dirs) == 0 {
			continue
		}
		b := Baseline{Experiment: exp, System: system, Reps: len(dirs)}
		var thr, g50, g95, g99, gt, p50, p95, p99, pt []float64
		for _, d := range dirs {
			r, err := LoadRun(d)
			if err != nil {
				return nil, err
			}
			b.Threads = r.Threads
			b.Errors += r.All.Errors()
			b.GenUtil = max(b.GenUtil, r.Generator.Utilization)
			thr = append(thr, r.Throughput)
			g50, g95, g99, gt = append(g50, r.Get.Latency.P50), append(g95, r.Get.Latency.P95), append(g99, r.Get.Latency.P99), append(gt, r.Get.Throughput)
			p50, p95, p99, pt = append(p50, r.Put.Latency.P50), append(p95, r.Put.Latency.P95), append(p99, r.Put.Latency.P99), append(pt, r.Put.Throughput)
		}
		b.Throughput = newStat(thr)
		b.Get = OpStat{newStat(gt), newStat(g50), newStat(g95), newStat(g99)}
		b.Put = OpStat{newStat(pt), newStat(p50), newStat(p95), newStat(p99)}
		if rtt, err := os.ReadFile(filepath.Join(dirs[0], "rtt.txt")); err == nil {
			b.RTT = lastLine(string(rtt))
		}
		out = append(out, b)
	}
	return out, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// BaselineTable builds the "latency per percentile" table of E2 and E4
// side by side: one row per system, experiment and operation.
func BaselineTable(e2, e4 []Baseline) Table {
	t := Table{Head: []string{"system", "experiment", "N", "reps", "op", "req/s", "p50 ms", "p95 ms", "p99 ms", "errors", "generator"}}
	for _, list := range [][]Baseline{e2, e4} {
		for _, b := range list {
			for _, op := range []struct {
				name string
				st   OpStat
			}{{"GET", b.Get}, {"PUT", b.Put}} {
				t.Add(b.System, b.Experiment, fmt.Sprint(b.Threads), fmt.Sprint(b.Reps), op.name,
					rng(op.st.Throughput, f0), rng(op.st.P50, f2), rng(op.st.P95, f2), rng(op.st.P99, f2),
					fmt.Sprint(b.Errors), fmt.Sprintf("%.0f%%", 100*b.GenUtil))
			}
		}
	}
	return t
}
