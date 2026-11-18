// Package results loads the files written by the experiments and combines
// the repetitions of each configuration.
package results

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
)

// Run is the summary.json of one run of the load generator, reduced to the
// fields the analysis uses.
type Run struct {
	System      string  `json:"system"`
	Read        string  `json:"read"`
	Threads     int     `json:"threads"`
	StartUnixNs int64   `json:"start_unix_ns"`
	DurationS   float64 `json:"duration_s"`
	WarmupS     float64 `json:"warmup_s"`
	WindowS     float64 `json:"window_s"`
	Throughput  float64 `json:"throughput"`
	All         Op      `json:"all"`
	Get         Op      `json:"get"`
	Put         Op      `json:"put"`
	Generator   struct {
		CPUs        int     `json:"cpus"`
		Cores       float64 `json:"cores"`
		Utilization float64 `json:"utilization"`
	} `json:"generator"`

	// NodeCPU is the number of CPUs each container kept busy during the
	// measured window, from cpu.csv. Empty if the file is missing.
	NodeCPU map[string]float64 `json:"-"`
}

// Op is the outcome of one operation in a run.
type Op struct {
	OK         int     `json:"ok"`
	Timeouts   int     `json:"timeouts"`
	Failed     int     `json:"failed"`
	Throughput float64 `json:"throughput"`
	Latency    struct {
		P50 float64 `json:"p50"`
		P95 float64 `json:"p95"`
		P99 float64 `json:"p99"`
	} `json:"latency_ms"`
}

// Errors is the number of requests that timed out or failed.
func (o Op) Errors() int { return o.Timeouts + o.Failed }

// LoadRun reads summary.json and, if present, cpu.csv from dir.
func LoadRun(dir string) (Run, error) {
	var r Run
	b, err := os.ReadFile(filepath.Join(dir, "summary.json"))
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("%s: %w", dir, err)
	}
	r.NodeCPU = loadCPU(filepath.Join(dir, "cpu.csv"), r)
	return r, nil
}

// loadCPU returns, per container, the CPU time consumed inside the measured
// window of r divided by the length of that window.
func loadCPU(path string, r Run) map[string]float64 {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil || len(rows) < 2 {
		return nil
	}
	from := r.StartUnixNs/1e6 + int64(r.WarmupS*1000)
	to := r.StartUnixNs/1e6 + int64(r.DurationS*1000)

	type point struct{ ms, usec int64 }
	first, last := map[string]point{}, map[string]point{}
	for _, row := range rows[1:] {
		if len(row) != 3 {
			continue
		}
		ms, err1 := strconv.ParseInt(row[0], 10, 64)
		usec, err2 := strconv.ParseInt(row[2], 10, 64)
		if err1 != nil || err2 != nil || ms < from || ms > to {
			continue
		}
		if _, ok := first[row[1]]; !ok {
			first[row[1]] = point{ms, usec}
		}
		last[row[1]] = point{ms, usec}
	}
	cpu := make(map[string]float64, len(first))
	for c, a := range first {
		if b := last[c]; b.ms > a.ms {
			cpu[c] = float64(b.usec-a.usec) / 1e6 / (float64(b.ms-a.ms) / 1e3)
		}
	}
	return cpu
}

// Stat is a value measured once per repetition.
type Stat struct {
	Median float64
	Min    float64
	Max    float64
}

func newStat(values []float64) Stat {
	if len(values) == 0 {
		return Stat{}
	}
	v := slices.Clone(values)
	slices.Sort(v)
	s := Stat{Min: v[0], Max: v[len(v)-1]}
	if mid := len(v) / 2; len(v)%2 == 1 {
		s.Median = v[mid]
	} else {
		s.Median = (v[mid-1] + v[mid]) / 2
	}
	return s
}

// Point is one thread count of the saturation curve, over its repetitions.
type Point struct {
	Threads    int
	Reps       int
	Throughput Stat // successful requests per second
	P99        Stat // latency of all operations, ms
	GetP99     Stat
	PutP99     Stat
	Errors     int                // requests that timed out or failed, summed over the repetitions
	NodeCPU    map[string]float64 // median busy CPUs per container
	GenUtil    float64            // highest generator CPU utilization among the repetitions

	// Ratio is the median throughput divided by the median p99 (requests
	// per second per millisecond). The optimal point maximizes it.
	Ratio float64
}

// Curve is the saturation curve of one system.
type Curve struct {
	System string
	Points []Point

	// Optimal is the index of the point with the highest Ratio (the knee).
	Optimal int
	// MaxThroughput is the highest median throughput of the curve.
	MaxThroughput float64
	// Plateau lists the indexes of the points whose median throughput is at
	// least plateauFraction of MaxThroughput, from the first of them on.
	Plateau []int
}

// plateauFraction defines the plateau: points within 5% of the maximum
// throughput.
const plateauFraction = 0.95

// LoadE1 reads results/e1/<system>/rep*/n*/ and returns one curve per system
// found, in alphabetical order.
func LoadE1(resultsDir string) ([]Curve, error) {
	dirs, err := filepath.Glob(filepath.Join(resultsDir, "e1", "*", "rep*", "n*", "summary.json"))
	if err != nil {
		return nil, err
	}
	bySystem := map[string]map[int][]Run{}
	for _, d := range dirs {
		runDir := filepath.Dir(d) // e1/<system>/rep<r>/n<N>
		run, err := LoadRun(runDir)
		if err != nil {
			return nil, err
		}
		system := filepath.Base(filepath.Dir(filepath.Dir(runDir)))
		if bySystem[system] == nil {
			bySystem[system] = map[int][]Run{}
		}
		bySystem[system][run.Threads] = append(bySystem[system][run.Threads], run)
	}

	var curves []Curve
	for system, byN := range bySystem {
		c := Curve{System: system}
		for n, runs := range byN {
			c.Points = append(c.Points, newPoint(n, runs))
		}
		sort.Slice(c.Points, func(i, j int) bool { return c.Points[i].Threads < c.Points[j].Threads })
		c.locate()
		curves = append(curves, c)
	}
	sort.Slice(curves, func(i, j int) bool { return curves[i].System < curves[j].System })
	return curves, nil
}

func newPoint(n int, runs []Run) Point {
	p := Point{Threads: n, Reps: len(runs), NodeCPU: map[string]float64{}}
	var thr, p99, get, put []float64
	cpu := map[string][]float64{}
	for _, r := range runs {
		thr = append(thr, r.Throughput)
		p99 = append(p99, r.All.Latency.P99)
		get = append(get, r.Get.Latency.P99)
		put = append(put, r.Put.Latency.P99)
		p.Errors += r.All.Errors()
		p.GenUtil = max(p.GenUtil, r.Generator.Utilization)
		for c, v := range r.NodeCPU {
			cpu[c] = append(cpu[c], v)
		}
	}
	p.Throughput, p.P99, p.GetP99, p.PutP99 = newStat(thr), newStat(p99), newStat(get), newStat(put)
	for c, v := range cpu {
		p.NodeCPU[c] = newStat(v).Median
	}
	if p.P99.Median > 0 {
		p.Ratio = p.Throughput.Median / p.P99.Median
	}
	return p
}

// locate finds the optimal point and the plateau of the curve.
func (c *Curve) locate() {
	for i, p := range c.Points {
		if p.Ratio > c.Points[c.Optimal].Ratio {
			c.Optimal = i
		}
		c.MaxThroughput = max(c.MaxThroughput, p.Throughput.Median)
	}
	c.Plateau = nil
	for i, p := range c.Points {
		if len(c.Plateau) > 0 || p.Throughput.Median >= plateauFraction*c.MaxThroughput {
			c.Plateau = append(c.Plateau, i)
		}
	}
}
