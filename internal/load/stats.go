package load

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"time"
)

// OpStats counts the requests of one operation that completed in an
// interval. The percentiles cover the successful requests only.
type OpStats struct {
	OK       int     `json:"ok"`
	Timeouts int     `json:"timeouts"`
	Failed   int     `json:"failed"`
	P50Ms    float64 `json:"p50_ms"`
	P99Ms    float64 `json:"p99_ms"`

	lats []uint32
}

func (o *OpStats) add(s Sample) {
	switch s.Status() {
	case OK:
		o.OK++
		o.lats = append(o.lats, s.Lat)
	case Timeout:
		o.Timeouts++
	default:
		o.Failed++
	}
}

func (o *OpStats) finish() {
	slices.Sort(o.lats)
	o.P50Ms = percentileMs(o.lats, 0.50)
	o.P99Ms = percentileMs(o.lats, 0.99)
	o.lats = nil
}

// NodeStats splits an interval by operation.
type NodeStats struct {
	Get OpStats `json:"get"`
	Put OpStats `json:"put"`
}

func (n *NodeStats) add(s Sample) {
	if s.Op() == Put {
		n.Put.add(s)
	} else {
		n.Get.add(s)
	}
}

// Interval is the statistics of the requests completed in one second.
type Interval struct {
	T      float64     `json:"t"`       // seconds since the run started
	UnixMs int64       `json:"unix_ms"` // the same instant as wall-clock time
	Warmup bool        `json:"warmup"`  // inside the warm-up period
	All    NodeStats   `json:"all"`     // every node
	Nodes  []NodeStats `json:"nodes"`   // per node that received the requests
}

func aggregate(samples []Sample, nodes int) Interval {
	iv := Interval{Nodes: make([]NodeStats, nodes)}
	for _, s := range samples {
		iv.All.add(s)
		if int(s.Node) < nodes {
			iv.Nodes[s.Node].add(s)
		}
	}
	iv.finish()
	return iv
}

func (iv *Interval) finish() {
	iv.All.Get.finish()
	iv.All.Put.finish()
	for i := range iv.Nodes {
		iv.Nodes[i].Get.finish()
		iv.Nodes[i].Put.finish()
	}
}

// percentileMs returns the q-quantile of sorted latencies, in milliseconds,
// by the nearest-rank method: the smallest value with at least a fraction q
// of the samples at or below it.
func percentileMs(sorted []uint32, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(q*float64(len(sorted)))) - 1
	i = max(0, min(i, len(sorted)-1))
	return float64(sorted[i]) / 1e6
}

// Latency summarizes the latency of successful requests, in milliseconds.
type Latency struct {
	Min  float64 `json:"min"`
	Mean float64 `json:"mean"`
	P50  float64 `json:"p50"`
	P90  float64 `json:"p90"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	P999 float64 `json:"p99.9"`
	Max  float64 `json:"max"`
}

// OpSummary is the outcome of one operation over the measured window. A
// request that timed out or failed has no latency: it only counts as an
// error.
type OpSummary struct {
	OK         int      `json:"ok"`
	Timeouts   int      `json:"timeouts"`
	Failed     int      `json:"failed"`
	Throughput float64  `json:"throughput"` // successful requests per second
	Latency    *Latency `json:"latency_ms,omitempty"`

	lats []uint32
}

func (o *OpSummary) add(s Sample) {
	switch s.Status() {
	case OK:
		o.OK++
		o.lats = append(o.lats, s.Lat)
	case Timeout:
		o.Timeouts++
	default:
		o.Failed++
	}
}

func (o *OpSummary) finish(window float64) {
	if window > 0 {
		o.Throughput = float64(o.OK) / window
	}
	if len(o.lats) > 0 {
		slices.Sort(o.lats)
		var sum float64
		for _, l := range o.lats {
			sum += float64(l)
		}
		o.Latency = &Latency{
			Min:  float64(o.lats[0]) / 1e6,
			Mean: sum / float64(len(o.lats)) / 1e6,
			P50:  percentileMs(o.lats, 0.50),
			P90:  percentileMs(o.lats, 0.90),
			P95:  percentileMs(o.lats, 0.95),
			P99:  percentileMs(o.lats, 0.99),
			P999: percentileMs(o.lats, 0.999),
			Max:  float64(o.lats[len(o.lats)-1]) / 1e6,
		}
	}
	o.lats = nil
}

// Errors returns the number of requests that did not succeed.
func (o OpSummary) Errors() int { return o.Timeouts + o.Failed }

// NodeSummary is the outcome of the requests received by one node.
type NodeSummary struct {
	Node string    `json:"node"`
	Get  OpSummary `json:"get"`
	Put  OpSummary `json:"put"`
}

// Generator reports how much CPU the load generator used inside the measured
// window. If Utilization approaches 1, the generator limits the throughput
// and the result does not describe the cluster.
type Generator struct {
	CPUs        int     `json:"cpus"`        // logical CPUs available to the generator
	CPUSeconds  float64 `json:"cpu_seconds"` // CPU time used
	Cores       float64 `json:"cores"`       // CPU time per second of window
	Utilization float64 `json:"utilization"` // Cores / CPUs
}

// Summary is the result of a run over its measured window, which starts at
// the end of the warm-up and ends with the run. Requests are assigned to the
// window by completion time.
type Summary struct {
	Start       time.Time `json:"start"`
	StartUnixNs int64     `json:"start_unix_ns"`
	Threads     int       `json:"threads"`
	DurationS   float64   `json:"duration_s"`
	WarmupS     float64   `json:"warmup_s"`
	WindowS     float64   `json:"window_s"`
	Keys        int       `json:"keys"`
	ValueBytes  int       `json:"value_bytes"`
	PutRatio    float64   `json:"put_ratio"`
	TimeoutMs   float64   `json:"timeout_ms"`
	Seed        uint64    `json:"seed"`
	Routing     string    `json:"routing"`

	Throughput float64       `json:"throughput"` // successful requests per second, all operations
	All        OpSummary     `json:"all"`
	Get        OpSummary     `json:"get"`
	Put        OpSummary     `json:"put"`
	Nodes      []NodeSummary `json:"nodes"`

	// Errors counts the error messages of the whole run, warm-up included.
	Errors    map[string]int `json:"errors,omitempty"`
	Generator Generator      `json:"generator"`
}

// NodeName is the name of node i in every output: n1, n2, n3.
func NodeName(i int) string { return "n" + strconv.Itoa(i+1) }

// Summarize computes the summary of a run.
func (r *Result) Summarize() Summary {
	cfg := r.Config
	from, to := int64(cfg.Warmup), int64(cfg.Duration)
	window := (cfg.Duration - cfg.Warmup).Seconds()

	s := Summary{
		Start:       r.Start,
		StartUnixNs: r.Start.UnixNano(),
		Threads:     cfg.Threads,
		DurationS:   cfg.Duration.Seconds(),
		WarmupS:     cfg.Warmup.Seconds(),
		WindowS:     window,
		Keys:        cfg.Keys,
		ValueBytes:  cfg.ValueSize,
		PutRatio:    cfg.PutRatio,
		TimeoutMs:   float64(cfg.Timeout) / 1e6,
		Seed:        cfg.Seed,
		Routing:     "rotation",
		Nodes:       make([]NodeSummary, r.Nodes),
		Errors:      r.errors,
	}
	if cfg.Pinned {
		s.Routing = "pinned"
	}
	for i := range s.Nodes {
		s.Nodes[i].Node = NodeName(i)
	}
	for _, rec := range r.recs {
		rec.each(func(_ int, sm Sample) {
			if end := sm.End(); end < from || end >= to {
				return
			}
			put := sm.Op() == Put
			s.All.add(sm)
			if put {
				s.Put.add(sm)
			} else {
				s.Get.add(sm)
			}
			if int(sm.Node) < len(s.Nodes) {
				if n := &s.Nodes[sm.Node]; put {
					n.Put.add(sm)
				} else {
					n.Get.add(sm)
				}
			}
		})
	}
	s.All.finish(window)
	s.Get.finish(window)
	s.Put.finish(window)
	for i := range s.Nodes {
		s.Nodes[i].Get.finish(window)
		s.Nodes[i].Put.finish(window)
	}
	s.Throughput = s.All.Throughput

	s.Generator = Generator{CPUs: CPUs(), CPUSeconds: r.cpu.Seconds()}
	if window > 0 {
		s.Generator.Cores = s.Generator.CPUSeconds / window
		s.Generator.Utilization = s.Generator.Cores / float64(s.Generator.CPUs)
	}
	return s
}

// Timeline returns the statistics of every second of the run, warm-up
// included, with requests assigned to the second in which they completed.
func (r *Result) Timeline() []Interval {
	seconds := int(math.Ceil(r.Config.Duration.Seconds()))
	tl := make([]Interval, seconds)
	for i := range tl {
		tl[i] = Interval{
			T:      float64(i),
			UnixMs: r.Start.Add(time.Duration(i) * time.Second).UnixMilli(),
			Warmup: time.Duration(i)*time.Second < r.Config.Warmup,
			Nodes:  make([]NodeStats, r.Nodes),
		}
	}
	for _, rec := range r.recs {
		rec.each(func(_ int, sm Sample) {
			sec := int(sm.End() / int64(time.Second))
			if sec < 0 || sec >= seconds {
				return
			}
			tl[sec].All.add(sm)
			if int(sm.Node) < r.Nodes {
				tl[sec].Nodes[sm.Node].add(sm)
			}
		})
	}
	for i := range tl {
		tl[i].finish()
	}
	return tl
}

// WriteTimelineCSV writes one row per second, node and operation. The node
// "all" aggregates every node.
func WriteTimelineCSV(w io.Writer, tl []Interval) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"second", "unix_s", "warmup", "node", "op", "ok", "timeouts", "failed", "p50_ms", "p99_ms"})
	row := func(iv Interval, node string, op Op, o OpStats) {
		cw.Write([]string{
			strconv.Itoa(int(iv.T)),
			strconv.FormatInt(iv.UnixMs/1000, 10),
			strconv.FormatBool(iv.Warmup),
			node,
			op.String(),
			strconv.Itoa(o.OK),
			strconv.Itoa(o.Timeouts),
			strconv.Itoa(o.Failed),
			strconv.FormatFloat(o.P50Ms, 'f', 3, 64),
			strconv.FormatFloat(o.P99Ms, 'f', 3, 64),
		})
	}
	for _, iv := range tl {
		row(iv, "all", Get, iv.All.Get)
		row(iv, "all", Put, iv.All.Put)
		for n, ns := range iv.Nodes {
			row(iv, NodeName(n), Get, ns.Get)
			row(iv, NodeName(n), Put, ns.Put)
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteSamplesCSV writes every sample of the run, gzip-compressed, one row
// per request. Together with the client number, seq identifies the value
// written by a PUT (see ParseStamp).
func (r *Result) WriteSamplesCSV(w io.Writer) error {
	zw := gzip.NewWriter(w)
	fmt.Fprintln(zw, "client,seq,start_unix_ns,latency_ns,op,key,node,status")
	base := r.Start.UnixNano()
	buf := make([]byte, 0, 96)
	var werr error
	for c, rec := range r.recs {
		rec.each(func(i int, sm Sample) {
			buf = buf[:0]
			buf = strconv.AppendInt(buf, int64(c), 10)
			buf = append(buf, ',')
			buf = strconv.AppendInt(buf, int64(i), 10)
			buf = append(buf, ',')
			buf = strconv.AppendInt(buf, base+sm.Start, 10)
			buf = append(buf, ',')
			buf = strconv.AppendUint(buf, uint64(sm.Lat), 10)
			buf = append(buf, ',')
			buf = append(buf, sm.Op().String()...)
			buf = append(buf, ',')
			buf = append(buf, KeyName(int(sm.Key))...)
			buf = append(buf, ',')
			buf = append(buf, NodeName(int(sm.Node))...)
			buf = append(buf, ',')
			buf = append(buf, sm.Status().String()...)
			buf = append(buf, '\n')
			if _, err := zw.Write(buf); err != nil && werr == nil {
				werr = err
			}
		})
	}
	if err := zw.Close(); err != nil && werr == nil {
		werr = err
	}
	return werr
}

// LatencyOf summarizes latencies given in nanoseconds. The slice is sorted
// in place.
func LatencyOf(lats []uint32) *Latency {
	o := OpSummary{lats: lats}
	o.finish(0)
	return o.Latency
}
