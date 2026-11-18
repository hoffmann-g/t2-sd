// Command stale is the coordinator of E3. It repeats, for a fixed number of
// pairs: PUT(k, v) on one node, with v increasing; when the PUT returns
// success, GET(k) on another node. The GET only starts after the PUT
// returned. A read is stale when the value it returns is older than the last
// value whose PUT succeeded. A PUT that fails or times out is not confirmed:
// the value it carried may or may not have been applied, so it does not
// raise the expected value.
//
// The value of pair i is the stamp "<run>:<i>:" followed by padding, where
// run identifies this execution of the coordinator, so a read value
// identifies the pair that wrote it (see load.ParseStamp). A value left by a
// previous execution is counted as stale with unknown lag.
//
//	docker exec gerador stale -system etcd -pairs 10000 -out /results/e3/etcd/normal
//
// With -out, the directory receives summary.json, timeline.csv (pairs and
// stale reads per second) and pairs.csv.gz (every pair).
package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"t2sd/internal/kv"
	"t2sd/internal/load"
)

type pair struct {
	Seq      int
	StartNs  int64 // PUT start, unix nanoseconds
	PutNs    int64 // PUT latency; 0 if the PUT failed
	PutOK    bool
	GetNs    int64 // GET latency; 0 if no GET was issued or it failed
	GetOK    bool
	ReadSeq  int  // pair whose value the GET returned; -1 if none, unparsable or from another run
	Expected int  // last confirmed pair at the time of the GET
	Stale    bool // ReadSeq < Expected
}

type summary struct {
	System    string  `json:"system"`
	Read      string  `json:"read"`
	WriteNode string  `json:"write_node"`
	ReadNode  string  `json:"read_node"`
	Key       string  `json:"key"`
	Pairs     int     `json:"pairs"`
	Start     int64   `json:"start_unix_ns"`
	DurationS float64 `json:"duration_s"`

	PutOK     int `json:"put_ok"`
	PutFailed int `json:"put_failed"`
	GetOK     int `json:"get_ok"`
	GetFailed int `json:"get_failed"`
	Stale     int `json:"stale"`
	// StaleRate is Stale divided by GetOK.
	StaleRate float64 `json:"stale_rate"`
	// MaxLag is the largest number of confirmed writes a stale read was
	// behind the expected value.
	MaxLag int `json:"max_lag"`
	// Missing counts GETs that found no value at all.
	Missing int `json:"missing"`

	PutLatencyMs *load.Latency `json:"put_latency_ms,omitempty"`
	GetLatencyMs *load.Latency `json:"get_latency_ms,omitempty"`
}

// interval is what the live mode prints each second and what timeline.csv
// stores.
type interval struct {
	T      float64 `json:"t"`
	UnixMs int64   `json:"unix_ms"`
	Pairs  int     `json:"pairs"`
	Stale  int     `json:"stale"`
	PutErr int     `json:"put_errors"`
	GetErr int     `json:"get_errors"`
	MaxLag int     `json:"max_lag"`
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("stale: ")
	system := flag.String("system", os.Getenv("LAB_SYSTEM"), "system under test: etcd or redis")
	nodes := flag.String("nodes", os.Getenv("LAB_NODES"), "client-network addresses of the nodes, comma-separated")
	pairs := flag.Int("pairs", 10000, "number of PUT/GET pairs")
	writeNode := flag.Int("write-node", 1, "node that receives the PUTs (1-based)")
	readNode := flag.Int("read-node", 3, "node that receives the GETs (1-based)")
	key := flag.String("key", "stale", "key written and read")
	valueSize := flag.Int("value", 256, "bytes per value")
	timeout := flag.Duration("timeout", 2*time.Second, "per-request timeout")
	read := flag.String("read", "", "etcd: linearizable (default) or serializable")
	out := flag.String("out", "", "directory for summary.json, timeline.csv and pairs.csv.gz")
	live := flag.Bool("live", false, "print the statistics of each second to stdout as JSON lines")
	flag.Parse()

	if *system == "" || *nodes == "" {
		flag.Usage()
		os.Exit(2)
	}
	addrs := strings.Split(*nodes, ",")
	if *writeNode < 1 || *writeNode > len(addrs) || *readNode < 1 || *readNode > len(addrs) {
		log.Fatalf("nodes must be between 1 and %d", len(addrs))
	}
	readMode := *read
	if *system == "etcd" && readMode == "" {
		readMode = "linearizable"
	}
	if *system == "redis" {
		readMode = "any node" // Get goes to the node named, Put to the primary
	}
	store, err := kv.Open(kv.Options{
		System:       *system,
		Addrs:        addrs,
		Timeout:      *timeout,
		Clients:      1,
		Serializable: readMode == "serializable",
		ReadFrom:     "all",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	value := make([]byte, *valueSize)
	for i := range value {
		value[i] = 'x'
	}
	wn, rn := *writeNode-1, *readNode-1
	res := make([]pair, 0, *pairs)
	start := time.Now()
	runID := int(start.Unix() % 100000)
	expected := -1 // last confirmed pair

	enc := json.NewEncoder(os.Stdout)
	var cur interval
	nextTick := start.Add(time.Second)
	flush := func(now time.Time) {
		for !now.Before(nextTick) {
			cur.T = nextTick.Sub(start).Seconds()
			cur.UnixMs = nextTick.UnixMilli()
			if *live {
				enc.Encode(cur)
			}
			timeline = append(timeline, cur)
			cur = interval{}
			nextTick = nextTick.Add(time.Second)
		}
	}

	for i := 0; i < *pairs && ctx.Err() == nil; i++ {
		p := pair{Seq: i, ReadSeq: -1}
		stampSeq(value, runID, i)

		t0 := time.Now()
		p.StartNs = t0.UnixNano()
		rctx, cancel := context.WithTimeout(ctx, *timeout)
		_, err := store.Put(rctx, wn, *key, value)
		cancel()
		t1 := time.Now()
		if err == nil {
			p.PutOK, p.PutNs = true, int64(t1.Sub(t0))
			expected = i
		} else {
			cur.PutErr++
		}
		p.Expected = expected

		if expected >= 0 {
			rctx, cancel := context.WithTimeout(ctx, *timeout)
			v, _, err := store.Get(rctx, rn, *key)
			cancel()
			t2 := time.Now()
			switch {
			case err != nil:
				cur.GetErr++
			case v == nil:
				p.GetOK, p.GetNs = true, int64(t2.Sub(t1))
				p.Stale = true
			default:
				p.GetOK, p.GetNs = true, int64(t2.Sub(t1))
				if run, seq, ok := load.ParseStamp(v); ok && run == runID {
					p.ReadSeq = int(seq)
				}
				p.Stale = p.ReadSeq < expected
			}
			if p.Stale {
				cur.Stale++
				cur.MaxLag = max(cur.MaxLag, expected-p.ReadSeq)
			}
		}
		cur.Pairs++
		res = append(res, p)
		flush(time.Now())
	}
	end := time.Now()
	flush(end.Add(time.Second))

	s := summarize(res, *system, readMode, wn, rn, *key, start, end)
	report := os.Stdout
	if *live {
		report = os.Stderr
	}
	fmt.Fprintf(report, "system=%s read=%s write=%s read-node=%s pairs=%d in %.1fs\n",
		s.System, s.Read, s.WriteNode, s.ReadNode, s.Pairs, s.DurationS)
	fmt.Fprintf(report, "PUT ok %d failed %d   GET ok %d failed %d   missing %d\n",
		s.PutOK, s.PutFailed, s.GetOK, s.GetFailed, s.Missing)
	fmt.Fprintf(report, "stale reads: %d of %d (%.3f%%), max lag %d confirmed writes\n",
		s.Stale, s.GetOK, 100*s.StaleRate, s.MaxLag)
	if s.PutLatencyMs != nil {
		fmt.Fprintf(report, "PUT p50 %.3f ms p99 %.3f ms   GET p50 %.3f ms p99 %.3f ms\n",
			s.PutLatencyMs.P50, s.PutLatencyMs.P99, s.GetLatencyMs.P50, s.GetLatencyMs.P99)
	}
	if *out != "" {
		if err := write(*out, s, res); err != nil {
			log.Fatal(err)
		}
	}
	if ctx.Err() != nil {
		os.Exit(1)
	}
}

var timeline []interval

// stampSeq writes the stamp of pair i, with the run id in the client field.
func stampSeq(value []byte, runID, i int) {
	copy(value, fmt.Sprintf("%05d:%012d:", runID, i))
}

func summarize(res []pair, system, readMode string, wn, rn int, key string, start, end time.Time) summary {
	s := summary{
		System: system, Read: readMode,
		WriteNode: load.NodeName(wn), ReadNode: load.NodeName(rn), Key: key,
		Pairs: len(res), Start: start.UnixNano(), DurationS: end.Sub(start).Seconds(),
	}
	var puts, gets []float64
	for _, p := range res {
		if p.PutOK {
			s.PutOK++
			puts = append(puts, float64(p.PutNs))
		} else {
			s.PutFailed++
		}
		if p.Expected < 0 {
			continue
		}
		if p.GetOK {
			s.GetOK++
			gets = append(gets, float64(p.GetNs))
		} else {
			s.GetFailed++
		}
		if p.GetOK && p.ReadSeq < 0 {
			s.Missing++
		}
		if p.Stale {
			s.Stale++
			s.MaxLag = max(s.MaxLag, p.Expected-p.ReadSeq)
		}
	}
	if s.GetOK > 0 {
		s.StaleRate = float64(s.Stale) / float64(s.GetOK)
	}
	s.PutLatencyMs = latency(puts)
	s.GetLatencyMs = latency(gets)
	return s
}

func latency(ns []float64) *load.Latency {
	if len(ns) == 0 {
		return nil
	}
	sorted := make([]uint32, len(ns))
	var sum float64
	for i, v := range ns {
		sorted[i] = uint32(min(v, float64(^uint32(0))))
		sum += v
	}
	// Reuse the percentile computation of the load package through a
	// one-operation summary.
	l := load.LatencyOf(sorted)
	l.Mean = sum / float64(len(ns)) / 1e6
	return l
}

func write(dir string, s summary, res []pair) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	tl, err := os.Create(filepath.Join(dir, "timeline.csv"))
	if err != nil {
		return err
	}
	fmt.Fprintln(tl, "second,unix_s,pairs,stale,put_errors,get_errors,max_lag")
	for _, iv := range timeline {
		fmt.Fprintf(tl, "%d,%d,%d,%d,%d,%d,%d\n", int(iv.T), iv.UnixMs/1000, iv.Pairs, iv.Stale, iv.PutErr, iv.GetErr, iv.MaxLag)
	}
	if err := tl.Close(); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "pairs.csv.gz"))
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(f)
	fmt.Fprintln(zw, "seq,start_unix_ns,put_ok,put_latency_ns,get_ok,get_latency_ns,read_seq,expected,stale")
	for _, p := range res {
		fmt.Fprintf(zw, "%d,%d,%t,%d,%t,%d,%d,%d,%t\n", p.Seq, p.StartNs, p.PutOK, p.PutNs, p.GetOK, p.GetNs, p.ReadSeq, p.Expected, p.Stale)
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}
