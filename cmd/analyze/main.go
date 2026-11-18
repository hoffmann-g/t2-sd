// Command analyze combines the repetitions of an experiment stored under
// results/ and prints its tables.
//
//	go run ./cmd/analyze e1
//
// E1 prints, for each system, one row per thread count N with the median
// over the repetitions (and the range between them) of the throughput and of
// the p99 latency, and identifies the optimal point and the saturation. It
// also writes results/e1/e1.csv with the same numbers.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"t2sd/internal/results"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("analyze: ")
	dir := flag.String("results", "results", "directory with the results of the experiments")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: analyze [-results dir] e1")
		flag.PrintDefaults()
	}
	flag.Parse()

	switch flag.Arg(0) {
	case "e1":
		if err := e1(*dir); err != nil {
			log.Fatal(err)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func e1(dir string) error {
	curves, err := results.LoadE1(dir)
	if err != nil {
		return err
	}
	if len(curves) == 0 {
		return fmt.Errorf("no E1 results under %s (run lab/scripts/e1.sh first)", filepath.Join(dir, "e1"))
	}
	for _, c := range curves {
		printCurve(c)
	}
	path := filepath.Join(dir, "e1", "e1.csv")
	if err := writeE1CSV(path, curves); err != nil {
		return err
	}
	fmt.Printf("written: %s\n", path)
	return nil
}

func printCurve(c results.Curve) {
	containers := containersOf(c)
	fmt.Printf("## E1: %s\n\n", c.System)

	head := []string{"N", "reps", "req/s", "range", "p99 ms", "range", "req/s per ms", "GET p99", "PUT p99", "errors"}
	for _, name := range containers {
		head = append(head, "CPU "+name)
	}
	head = append(head, "generator")
	fmt.Println("| " + strings.Join(head, " | ") + " |")
	fmt.Println("|" + strings.Repeat(" ---: |", len(head)))

	for i, p := range c.Points {
		mark := ""
		if i == c.Optimal {
			mark = " *"
		}
		row := []string{
			strconv.Itoa(p.Threads) + mark,
			strconv.Itoa(p.Reps),
			fmt.Sprintf("%.0f", p.Throughput.Median),
			fmt.Sprintf("%.0f to %.0f", p.Throughput.Min, p.Throughput.Max),
			fmt.Sprintf("%.2f", p.P99.Median),
			fmt.Sprintf("%.2f to %.2f", p.P99.Min, p.P99.Max),
			fmt.Sprintf("%.0f", p.Ratio),
			fmt.Sprintf("%.2f", p.GetP99.Median),
			fmt.Sprintf("%.2f", p.PutP99.Median),
			strconv.Itoa(p.Errors),
		}
		for _, name := range containers {
			row = append(row, fmt.Sprintf("%.2f", p.NodeCPU[name]))
		}
		row = append(row, fmt.Sprintf("%.0f%%", 100*p.GenUtil))
		fmt.Println("| " + strings.Join(row, " | ") + " |")
	}

	opt := c.Points[c.Optimal]
	fmt.Printf("\nOptimal point (*): N = %d, %.0f req/s at p99 %.2f ms (%.0f req/s per ms).\n",
		opt.Threads, opt.Throughput.Median, opt.P99.Median, opt.Ratio)
	fmt.Printf("Maximum sustained throughput: %.0f req/s. Plateau (within 5%% of it) from N = %d, %d point(s).\n",
		c.MaxThroughput, c.Points[c.Plateau[0]].Threads, len(c.Plateau))

	for _, w := range warnings(c) {
		fmt.Println("WARNING: " + w)
	}
	fmt.Println()
}

// warnings lists what makes the curve unfit for the report.
func warnings(c results.Curve) []string {
	var w []string
	if len(c.Plateau) < 2 {
		w = append(w, "fewer than two points on the plateau: extend the curve with larger N")
	}
	var fewReps, errs, busy []string
	for _, p := range c.Points {
		n := strconv.Itoa(p.Threads)
		if p.Reps < 3 {
			fewReps = append(fewReps, n)
		}
		if p.Errors > 0 {
			errs = append(errs, n)
		}
		if p.GenUtil > 0.8 {
			busy = append(busy, n)
		}
	}
	if len(fewReps) > 0 {
		w = append(w, "fewer than 3 repetitions at N = "+strings.Join(fewReps, ", "))
	}
	if len(errs) > 0 {
		w = append(w, "requests failed or timed out at N = "+strings.Join(errs, ", "))
	}
	if len(busy) > 0 {
		w = append(w, "generator above 80% of its CPUs at N = "+strings.Join(busy, ", ")+": the throughput there may reflect the generator")
	}
	return w
}

func containersOf(c results.Curve) []string {
	seen := map[string]bool{}
	for _, p := range c.Points {
		for name := range p.NodeCPU {
			if name != "gerador" {
				seen[name] = true
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func writeE1CSV(path string, curves []results.Curve) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{
		"system", "threads", "reps",
		"throughput_median", "throughput_min", "throughput_max",
		"p99_ms_median", "p99_ms_min", "p99_ms_max",
		"get_p99_ms", "put_p99_ms", "throughput_per_ms", "errors",
		"generator_utilization", "optimal", "plateau",
	})
	g := func(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }
	for _, c := range curves {
		plateau := map[int]bool{}
		for _, i := range c.Plateau {
			plateau[i] = true
		}
		for i, p := range c.Points {
			w.Write([]string{
				c.System, strconv.Itoa(p.Threads), strconv.Itoa(p.Reps),
				g(p.Throughput.Median), g(p.Throughput.Min), g(p.Throughput.Max),
				g(p.P99.Median), g(p.P99.Min), g(p.P99.Max),
				g(p.GetP99.Median), g(p.PutP99.Median), g(p.Ratio), strconv.Itoa(p.Errors),
				g(p.GenUtil), strconv.FormatBool(i == c.Optimal), strconv.FormatBool(plateau[i]),
			})
		}
	}
	w.Flush()
	return w.Error()
}
