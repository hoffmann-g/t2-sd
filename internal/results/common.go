package results

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Event is one line of events.csv: an instant of the run marked by the
// script (partition, heal, kill, start).
type Event struct {
	UnixMs int64
	Label  string
}

// LoadEvents reads events.csv from dir. A missing file gives no events.
func LoadEvents(dir string) ([]Event, error) {
	f, err := os.Open(filepath.Join(dir, "events.csv"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	var evs []Event
	for _, row := range rows[1:] {
		if len(row) != 2 {
			continue
		}
		ms, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			continue
		}
		evs = append(evs, Event{ms, row[1]})
	}
	return evs, nil
}

// Find returns the instant of the first event whose label starts with
// prefix, in unix milliseconds, and whether it exists.
func Find(evs []Event, prefix string) (int64, string, bool) {
	for _, e := range evs {
		if strings.HasPrefix(e.Label, prefix) {
			return e.UnixMs, e.Label, true
		}
	}
	return 0, "", false
}

// Second is one row of timeline.csv.
type Second struct {
	Sec      int
	UnixS    int64
	Warmup   bool
	Node     string // "all", "n1", "n2", "n3"
	Op       string // "GET" or "PUT"
	OK       int
	Timeouts int
	Failed   int
	P50Ms    float64
	P99Ms    float64
}

func (s Second) Errors() int { return s.Timeouts + s.Failed }

// LoadTimeline reads timeline.csv from dir.
func LoadTimeline(dir string) ([]Second, error) {
	f, err := os.Open(filepath.Join(dir, "timeline.csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	var tl []Second
	for _, row := range rows[1:] {
		if len(row) != 10 {
			continue
		}
		var s Second
		s.Sec, _ = strconv.Atoi(row[0])
		s.UnixS, _ = strconv.ParseInt(row[1], 10, 64)
		s.Warmup = row[2] == "true"
		s.Node, s.Op = row[3], row[4]
		s.OK, _ = strconv.Atoi(row[5])
		s.Timeouts, _ = strconv.Atoi(row[6])
		s.Failed, _ = strconv.Atoi(row[7])
		s.P50Ms, _ = strconv.ParseFloat(row[8], 64)
		s.P99Ms, _ = strconv.ParseFloat(row[9], 64)
		tl = append(tl, s)
	}
	return tl, nil
}

// SampleRow is one row of samples.csv.gz.
type SampleRow struct {
	Client  int
	Seq     uint64
	StartNs int64 // unix nanoseconds
	LatNs   int64
	Op      string
	Key     string
	Node    string
	Status  string // ok, timeout, failed
}

func (s SampleRow) EndNs() int64 { return s.StartNs + s.LatNs }

// EachSample calls f for every row of samples.csv.gz in dir.
func EachSample(dir string, f func(SampleRow)) error {
	file, err := os.Open(filepath.Join(dir, "samples.csv.gz"))
	if err != nil {
		return err
	}
	defer file.Close()
	zr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ",")
		if len(fields) != 8 || fields[0] == "client" {
			continue
		}
		var s SampleRow
		s.Client, _ = strconv.Atoi(fields[0])
		s.Seq, _ = strconv.ParseUint(fields[1], 10, 64)
		s.StartNs, _ = strconv.ParseInt(fields[2], 10, 64)
		s.LatNs, _ = strconv.ParseInt(fields[3], 10, 64)
		s.Op, s.Key, s.Node, s.Status = fields[4], fields[5], fields[6], fields[7]
		f(s)
	}
	return sc.Err()
}

// repDirs lists the rep* directories under dir that contain file, sorted.
func repDirs(dir, file string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "rep*", file))
	if err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(matches))
	for _, m := range matches {
		dirs = append(dirs, filepath.Dir(m))
	}
	sort.Strings(dirs)
	return dirs, nil
}

// subdirs lists the directories directly under dir, sorted.
func subdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// Table is a Markdown table that is also written as CSV.
type Table struct {
	Head []string
	Rows [][]string
}

func (t *Table) Add(cells ...string) { t.Rows = append(t.Rows, cells) }

func (t Table) WriteMarkdown(w io.Writer) {
	fmt.Fprintln(w, "| "+strings.Join(t.Head, " | ")+" |")
	fmt.Fprintln(w, "|"+strings.Repeat(" ---: |", len(t.Head)))
	for _, r := range t.Rows {
		fmt.Fprintln(w, "| "+strings.Join(r, " | ")+" |")
	}
}

func (t Table) WriteCSV(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write(t.Head)
	for _, r := range t.Rows {
		w.Write(r)
	}
	w.Flush()
	return w.Error()
}

func f0(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) }
func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
func f3(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

// rng formats a Stat as "median (min to max)".
func rng(s Stat, f func(float64) string) string {
	if s.Min == s.Max {
		return f(s.Median)
	}
	return fmt.Sprintf("%s (%s to %s)", f(s.Median), f(s.Min), f(s.Max))
}
