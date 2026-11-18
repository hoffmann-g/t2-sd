// Command loadgen is the closed-loop load generator of the lab.
//
// It starts N clients. Each client repeats, with no pause between requests:
// draw an operation (20% PUT, 80% GET by default) and a key, uniformly among
// 10,000 keys; send the request to one node and wait for the answer or the
// timeout; record a sample with the send time, the latency, the operation,
// the node and the outcome. A request that times out or fails has no
// latency: it only counts as an error.
//
// The draws depend only on -seed and on the client number, so two runs with
// the same parameters issue the same sequence of operations per client.
//
// It runs inside the "gerador" container, which reaches the nodes through
// the client network:
//
//	docker exec gerador loadgen -system etcd -threads 16 -out /results/demo
//
// With -out, the directory receives summary.json (throughput, latency
// percentiles and errors over the measured window), timeline.csv (requests
// completed per second, per node and operation) and, with -samples,
// samples.csv.gz (every PUT and every failed request; -samples-all writes
// every request).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"t2sd/internal/kv"
	"t2sd/internal/load"
)

// output is the content of summary.json.
type output struct {
	System string   `json:"system"`
	Addrs  []string `json:"addrs"`
	Read   string   `json:"read"`
	Conns  int      `json:"conns_per_node,omitempty"`
	load.Summary
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("loadgen: ")

	system := flag.String("system", os.Getenv("LAB_SYSTEM"), "system under test: etcd or redis")
	nodes := flag.String("nodes", os.Getenv("LAB_NODES"), "client-network addresses of the nodes, comma-separated, in node order")
	threads := flag.Int("threads", 1, "number of clients (N)")
	duration := flag.Duration("duration", 2*time.Minute, "length of the run, warm-up included")
	warmup := flag.Duration("warmup", 30*time.Second, "initial period left out of the statistics")
	keys := flag.Int("keys", 10000, "number of keys")
	valueSize := flag.Int("value", 256, "bytes per value")
	putRatio := flag.Float64("put", 0.2, "fraction of requests that are PUT")
	timeout := flag.Duration("timeout", 2*time.Second, "per-request timeout")
	seed := flag.Uint64("seed", 1, "seed of the operation and key draws")
	pinned := flag.Bool("pinned", false, "client i sends every request through node i mod N, instead of rotating over the nodes")
	nodeRetry := flag.Duration("node-retry", time.Second, "rotation: time a node is skipped after a failed request")
	errorPause := flag.Duration("error-pause", 10*time.Millisecond, "minimum time from the start of a failed request to the next request of the client")
	read := flag.String("read", "", "read mode. etcd: linearizable (default) or serializable. redis: replicas (default), all or primary")
	conns := flag.Int("conns", 0, "etcd: gRPC connections per node (default: one per 16 clients, at most 16)")
	preload := flag.Bool("preload", true, "write every key once before the run")
	out := flag.String("out", "", "directory for summary.json and timeline.csv")
	samples := flag.Bool("samples", false, "with -out, also write samples.csv.gz with every PUT and every failed request")
	samplesAll := flag.Bool("samples-all", false, "like -samples, but with every request")
	live := flag.Bool("live", false, "print the statistics of each second to stdout as JSON lines")
	flag.Parse()

	if *system == "" || *nodes == "" {
		log.Print("-system and -nodes are required (or LAB_SYSTEM and LAB_NODES)")
		flag.Usage()
		os.Exit(2)
	}
	addrs := strings.Split(*nodes, ",")

	opts := kv.Options{
		System:  *system,
		Addrs:   addrs,
		Timeout: *timeout,
		Clients: *threads,
	}
	readMode := *read
	switch *system {
	case "etcd":
		if readMode == "" {
			readMode = "linearizable"
		}
		if readMode != "linearizable" && readMode != "serializable" {
			log.Fatalf("etcd: unknown read mode %q", readMode)
		}
		opts.Serializable = readMode == "serializable"
		opts.Conns = *conns
		if opts.Conns < 1 {
			opts.Conns = min(16, (*threads+15)/16)
		}
	case "redis":
		if readMode == "" {
			readMode = "replicas"
		}
		opts.ReadFrom = readMode
	}

	store, err := kv.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	cfg := load.Config{
		Threads:    *threads,
		Duration:   *duration,
		Warmup:     *warmup,
		Keys:       *keys,
		ValueSize:  *valueSize,
		PutRatio:   *putRatio,
		Timeout:    *timeout,
		Seed:       *seed,
		Pinned:     *pinned,
		NodeRetry:  *nodeRetry,
		ErrorPause: *errorPause,
	}
	if *live {
		enc := json.NewEncoder(os.Stdout)
		cfg.Live = func(iv load.Interval) { enc.Encode(iv) }
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *preload {
		if err := load.Preload(ctx, store, cfg); err != nil {
			log.Fatal(err)
		}
	}

	res, err := load.Run(ctx, store, cfg)
	if res == nil {
		log.Fatal(err)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

	o := output{System: *system, Addrs: addrs, Read: readMode, Conns: opts.Conns, Summary: res.Summarize()}
	report := os.Stdout
	if *live {
		report = os.Stderr
	}
	printSummary(report, o)

	if *out != "" {
		if err := writeOutputs(*out, o, res, *samples || *samplesAll, *samplesAll); err != nil {
			log.Fatal(err)
		}
	}
}

func printSummary(w *os.File, o output) {
	s := o.Summary
	fmt.Fprintf(w, "system=%s read=%s threads=%d routing=%s window=%.0fs (warm-up %.0fs) seed=%d\n",
		o.System, o.Read, s.Threads, s.Routing, s.WindowS, s.WarmupS, s.Seed)
	fmt.Fprintf(w, "throughput %.1f req/s   errors %d (%d timeouts)\n",
		s.Throughput, s.All.Errors(), s.All.Timeouts)
	fmt.Fprintf(w, "%-4s %10s %10s %8s %9s %9s %9s %9s %9s\n",
		"", "ok", "req/s", "errors", "p50 ms", "p95 ms", "p99 ms", "p99.9 ms", "max ms")
	row := func(name string, op load.OpSummary) {
		l := op.Latency
		if l == nil {
			l = &load.Latency{}
		}
		fmt.Fprintf(w, "%-4s %10d %10.1f %8d %9.3f %9.3f %9.3f %9.3f %9.3f\n",
			name, op.OK, op.Throughput, op.Errors(), l.P50, l.P95, l.P99, l.P999, l.Max)
	}
	row("GET", s.Get)
	row("PUT", s.Put)
	row("all", s.All)
	g := s.Generator
	fmt.Fprintf(w, "generator CPU: %.2f of %d CPUs (%.0f%%)\n", g.Cores, g.CPUs, 100*g.Utilization)
	if len(s.Errors) > 0 {
		msgs := make([]string, 0, len(s.Errors))
		for m := range s.Errors {
			msgs = append(msgs, m)
		}
		sort.Slice(msgs, func(i, j int) bool { return s.Errors[msgs[i]] > s.Errors[msgs[j]] })
		fmt.Fprintln(w, "errors in the whole run:")
		for _, m := range msgs[:min(len(msgs), 8)] {
			fmt.Fprintf(w, "  %7d  %s\n", s.Errors[m], m)
		}
	}
}

func writeOutputs(dir string, o output, res *load.Result, samples, all bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(dir, "timeline.csv"))
	if err != nil {
		return err
	}
	if err := load.WriteTimelineCSV(f, res.Timeline()); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if samples {
		f, err := os.Create(filepath.Join(dir, "samples.csv.gz"))
		if err != nil {
			return err
		}
		if err := res.WriteSamplesCSV(f, all); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	return nil
}
