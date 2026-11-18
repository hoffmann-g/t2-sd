// Command check compares the state of the nodes with the writes a load run
// confirmed. It is used after the partition of E5 is healed.
//
// It reads every key from every node and looks up, in the samples of the run
// (samples.csv.gz), the PUT that wrote the value found. It reports:
//
//   - divergence: keys whose nodes hold different values;
//   - lost writes: keys whose stored value comes from a PUT that completed
//     before another confirmed PUT on the same key started. That later PUT
//     was acknowledged and then discarded, or was applied before an older
//     one;
//   - unknown values: values that match no confirmed PUT of the run, for
//     example a write that failed or timed out but was applied anyway.
//
// Two PUTs that overlap in time may be applied in either order, so neither
// counts as lost.
//
//	docker exec gerador check -samples /results/e5/etcd/rep1/samples.csv.gz
package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"t2sd/internal/kv"
	"t2sd/internal/load"
)

type write struct {
	key        string
	start, end int64
}

type report struct {
	Keys      int            `json:"keys"`
	Nodes     []string       `json:"nodes"`
	Divergent int            `json:"divergent_keys"`
	Lost      map[string]int `json:"lost_writes_per_node"`
	Unknown   map[string]int `json:"unknown_values_per_node"`
	Missing   map[string]int `json:"missing_keys_per_node"`
	Errors    map[string]int `json:"read_errors_per_node"`
	Examples  []string       `json:"examples"`
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("check: ")
	system := flag.String("system", os.Getenv("LAB_SYSTEM"), "system under test: etcd or redis")
	nodes := flag.String("nodes", os.Getenv("LAB_NODES"), "client-network addresses of the nodes, comma-separated")
	samples := flag.String("samples", "", "samples.csv.gz of the run")
	keys := flag.Int("keys", 10000, "number of keys")
	timeout := flag.Duration("timeout", 2*time.Second, "per-request timeout")
	out := flag.String("out", "", "write the report as JSON to this file")
	flag.Parse()
	if *system == "" || *nodes == "" || *samples == "" {
		flag.Usage()
		os.Exit(2)
	}

	writes, maxStart, err := loadWrites(*samples)
	if err != nil {
		log.Fatal(err)
	}
	addrs := strings.Split(*nodes, ",")
	store, err := kv.Open(kv.Options{System: *system, Addrs: addrs, Timeout: *timeout, Clients: 1, ReadFrom: "all", Serializable: true})
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	r := report{Keys: *keys, Lost: map[string]int{}, Unknown: map[string]int{}, Missing: map[string]int{}, Errors: map[string]int{}}
	for i := range addrs {
		r.Nodes = append(r.Nodes, load.NodeName(i))
	}
	example := func(format string, args ...any) {
		if len(r.Examples) < 20 {
			r.Examples = append(r.Examples, fmt.Sprintf(format, args...))
		}
	}
	for k := 0; k < *keys; k++ {
		key := load.KeyName(k)
		values := make([]string, len(addrs))
		for node := range addrs {
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			v, _, err := store.Get(ctx, node, key)
			cancel()
			name := load.NodeName(node)
			switch {
			case err != nil:
				r.Errors[name]++
				values[node] = "error"
				continue
			case v == nil:
				r.Missing[name]++
				values[node] = "missing"
				continue
			}
			client, seq, ok := load.ParseStamp(v)
			if !ok {
				r.Unknown[name]++
				values[node] = "unparsable"
				continue
			}
			values[node] = fmt.Sprintf("%d:%d", client, seq)
			w, found := writes[stampKey{client, seq}]
			if !found {
				if maxStart[key] > 0 { // the run wrote this key, but not this value
					r.Unknown[name]++
					example("%s on %s holds %s, which is not a confirmed write of the run", key, name, values[node])
				}
				continue
			}
			if maxStart[key] > w.end {
				r.Lost[name]++
				example("%s on %s holds the write %s (ended %.3f s), but a write confirmed later started at %.3f s",
					key, name, values[node], float64(w.end)/1e9, float64(maxStart[key])/1e9)
			}
		}
		if diverge(values) {
			r.Divergent++
			example("%s differs between nodes: %s", key, strings.Join(values, " "))
		}
	}

	fmt.Printf("keys %d, nodes %s\n", r.Keys, strings.Join(r.Nodes, " "))
	fmt.Printf("divergent keys: %d\n", r.Divergent)
	for _, n := range r.Nodes {
		fmt.Printf("%s: lost writes %d, unknown values %d, missing keys %d, read errors %d\n",
			n, r.Lost[n], r.Unknown[n], r.Missing[n], r.Errors[n])
	}
	if len(r.Examples) > 0 {
		fmt.Println("examples:")
		for _, e := range r.Examples {
			fmt.Println("  " + e)
		}
	}
	if *out != "" {
		b, _ := json.MarshalIndent(r, "", "  ")
		if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

type stampKey struct {
	client int
	seq    uint64
}

// loadWrites returns the confirmed PUTs of the run, by stamp, and the latest
// start time of a confirmed PUT on each key, relative to the first sample.
func loadWrites(path string) (map[stampKey]write, map[string]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, err
	}
	writes := map[stampKey]write{}
	maxStart := map[string]int64{}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var base int64 = -1
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ",")
		if len(fields) != 8 || fields[4] != "PUT" || fields[7] != "ok" {
			continue
		}
		client, _ := strconv.Atoi(fields[0])
		seq, _ := strconv.ParseUint(fields[1], 10, 64)
		start, _ := strconv.ParseInt(fields[2], 10, 64)
		lat, _ := strconv.ParseInt(fields[3], 10, 64)
		if base < 0 {
			base = start
		}
		w := write{key: fields[5], start: start - base, end: start - base + lat}
		writes[stampKey{client, seq}] = w
		maxStart[w.key] = max(maxStart[w.key], w.start)
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}
	if len(writes) == 0 {
		return nil, nil, fmt.Errorf("%s: no confirmed PUT found", path)
	}
	return writes, maxStart, nil
}

func diverge(values []string) bool {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted[0] != sorted[len(sorted)-1]
}
