package load

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

// fakeStore is an in-memory store with three nodes that share one map.
type fakeStore struct {
	mu    sync.Mutex
	data  map[string][]byte
	calls []call // every request, in arrival order

	down  map[int]bool // nodes that refuse requests
	stuck map[int]bool // nodes that never answer
	read  []int
	write []int
}

type call struct {
	op   Op
	node int
	key  string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		data:  map[string][]byte{},
		down:  map[int]bool{},
		stuck: map[int]bool{},
		read:  []int{0, 1, 2},
		write: []int{0, 1, 2},
	}
}

var errRefused = errors.New("connection refused")

func (f *fakeStore) Nodes() int        { return 3 }
func (f *fakeStore) ReadNodes() []int  { return f.read }
func (f *fakeStore) WriteNodes() []int { return f.write }
func (f *fakeStore) Close() error      { return nil }

func (f *fakeStore) begin(ctx context.Context, op Op, node int, key string) error {
	f.mu.Lock()
	f.calls = append(f.calls, call{op, node, key})
	down, stuck := f.down[node], f.stuck[node]
	f.mu.Unlock()
	if stuck {
		<-ctx.Done()
		return ctx.Err()
	}
	if down {
		return errRefused
	}
	return nil
}

func (f *fakeStore) Put(ctx context.Context, node int, key string, value []byte) (int, error) {
	if err := f.begin(ctx, Put, node, key); err != nil {
		return node, err
	}
	f.mu.Lock()
	f.data[key] = append([]byte(nil), value...)
	f.mu.Unlock()
	return node, nil
}

func (f *fakeStore) Get(ctx context.Context, node int, key string) ([]byte, int, error) {
	if err := f.begin(ctx, Get, node, key); err != nil {
		return nil, node, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data[key], node, nil
}

func testConfig() Config {
	return Config{
		Threads:    6,
		Duration:   300 * time.Millisecond,
		Warmup:     100 * time.Millisecond,
		Keys:       100,
		ValueSize:  64,
		PutRatio:   0.2,
		Timeout:    50 * time.Millisecond,
		Seed:       7,
		ErrorPause: time.Millisecond,
		NodeRetry:  20 * time.Millisecond,
	}
}

func TestPercentileNearestRank(t *testing.T) {
	sorted := []uint32{1e6, 2e6, 3e6, 4e6, 5e6, 6e6, 7e6, 8e6, 9e6, 10e6}
	for _, c := range []struct {
		q    float64
		want float64
	}{{0.50, 5}, {0.90, 9}, {0.99, 10}, {0.0, 1}, {1.0, 10}} {
		if got := percentileMs(sorted, c.q); got != c.want {
			t.Errorf("q=%.2f: got %v ms, want %v", c.q, got, c.want)
		}
	}
	if got := percentileMs(nil, 0.99); got != 0 {
		t.Errorf("empty input: got %v, want 0", got)
	}
}

func TestSampleFields(t *testing.T) {
	s := newSample(3*time.Second, 7*time.Millisecond, 9999, 2, Put, Timeout)
	if s.Op() != Put || s.Status() != Timeout || s.Key != 9999 || s.Node != 2 {
		t.Fatalf("fields lost: %+v op=%v status=%v", s, s.Op(), s.Status())
	}
	if want := int64(3*time.Second + 7*time.Millisecond); s.End() != want {
		t.Errorf("End = %d, want %d", s.End(), want)
	}
	if long := newSample(0, 10*time.Second, 0, 0, Get, OK); long.Lat != math.MaxUint32 {
		t.Errorf("latency above the limit must saturate, got %d", long.Lat)
	}
}

func TestStampRoundTrip(t *testing.T) {
	buf := make([]byte, 64)
	stamp(buf, 511, 123456789)
	if string(buf[:stampLen]) != "00511:000123456789:" {
		t.Fatalf("stamp = %q", buf[:stampLen])
	}
	client, seq, ok := ParseStamp(buf)
	if !ok || client != 511 || seq != 123456789 {
		t.Errorf("ParseStamp = %d, %d, %v", client, seq, ok)
	}
	if _, _, ok := ParseStamp([]byte("not a stamped value")); ok {
		t.Error("ParseStamp accepted a value without a stamp")
	}
}

func TestRunMixAndRotation(t *testing.T) {
	store := newFakeStore()
	res, err := Run(context.Background(), store, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	s := res.Summarize()
	if s.All.OK == 0 || s.All.Errors() != 0 {
		t.Fatalf("ok=%d errors=%d", s.All.OK, s.All.Errors())
	}
	if s.All.OK != s.Get.OK+s.Put.OK {
		t.Errorf("all=%d but get+put=%d", s.All.OK, s.Get.OK+s.Put.OK)
	}
	if ratio := float64(s.Put.OK) / float64(s.All.OK); math.Abs(ratio-0.2) > 0.03 {
		t.Errorf("PUT fraction = %.3f, want about 0.20", ratio)
	}
	if got, want := s.Throughput, float64(s.All.OK)/s.WindowS; got != want {
		t.Errorf("throughput = %v, want %v", got, want)
	}
	// Rotation spreads each operation evenly over the nodes.
	for _, n := range s.Nodes {
		share := float64(n.Get.OK+n.Put.OK) / float64(s.All.OK)
		if math.Abs(share-1.0/3) > 0.02 {
			t.Errorf("%s received %.3f of the requests, want about 1/3", n.Node, share)
		}
	}
	if s.Get.Latency == nil || s.Get.Latency.P99 < s.Get.Latency.P50 {
		t.Errorf("latency summary inconsistent: %+v", s.Get.Latency)
	}
}

func TestRunSameSeedSameDraws(t *testing.T) {
	draws := func(seed uint64) []call {
		store := newFakeStore()
		cfg := testConfig()
		cfg.Threads, cfg.Seed = 1, seed
		if _, err := Run(context.Background(), store, cfg); err != nil {
			t.Fatal(err)
		}
		return store.calls
	}
	a, b, other := draws(7), draws(7), draws(8)
	n := min(len(a), len(b), len(other), 500)
	if n < 100 {
		t.Fatalf("only %d requests to compare", n)
	}
	differs := false
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			t.Fatalf("request %d differs between runs with the same seed: %v and %v", i, a[i], b[i])
		}
		if a[i] != other[i] {
			differs = true
		}
	}
	if !differs {
		t.Error("a different seed produced the same requests")
	}
}

func TestRunPinned(t *testing.T) {
	store := newFakeStore()
	cfg := testConfig()
	cfg.Pinned = true
	res, err := Run(context.Background(), store, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for client, rec := range res.recs {
		rec.each(func(_ int, s Sample) {
			if int(s.Node) != client%3 {
				t.Fatalf("client %d sent a request to node %d", client, s.Node)
			}
		})
	}
	cfg.Threads = 2
	if _, err := Run(context.Background(), store, cfg); err == nil {
		t.Error("pinned mode accepted fewer clients than nodes")
	}
}

func TestRunTimeoutHasNoLatency(t *testing.T) {
	store := newFakeStore()
	store.stuck[0] = true
	cfg := testConfig()
	cfg.Pinned = true // rotation would stop sending to the stuck node
	res, err := Run(context.Background(), store, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := res.Summarize()
	n1 := s.Nodes[0]
	if n1.Get.OK+n1.Put.OK != 0 || n1.Get.Timeouts+n1.Put.Timeouts == 0 {
		t.Fatalf("stuck node: %+v", n1)
	}
	if n1.Get.Latency != nil || n1.Put.Latency != nil {
		t.Error("timed-out requests must not contribute latency")
	}
	if n1.Get.Failed+n1.Put.Failed != 0 {
		t.Error("timeouts counted as failures")
	}
	if s.Nodes[1].Get.OK == 0 || s.Nodes[2].Get.OK == 0 {
		t.Error("clients of the other nodes were affected")
	}
	if s.All.Timeouts != s.Get.Timeouts+s.Put.Timeouts || s.All.Timeouts == 0 {
		t.Errorf("timeouts: all=%d get=%d put=%d", s.All.Timeouts, s.Get.Timeouts, s.Put.Timeouts)
	}
}

func TestRunRotationAvoidsFailedNode(t *testing.T) {
	store := newFakeStore()
	store.down[1] = true
	cfg := testConfig()
	cfg.Duration, cfg.Warmup = 400*time.Millisecond, 0
	res, err := Run(context.Background(), store, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := res.Summarize()
	failed := s.Nodes[1].Get.Failed + s.Nodes[1].Put.Failed
	if failed == 0 {
		t.Fatal("no request reached the failed node")
	}
	// One probe per retry interval (20 ms) over 400 ms, plus the requests
	// in flight when the first failure was reported.
	if limit := 400/20 + cfg.Threads + 2; failed > limit {
		t.Errorf("%d requests sent to the failed node, want at most %d", failed, limit)
	}
	if s.Nodes[0].Get.OK == 0 || s.Nodes[2].Get.OK == 0 {
		t.Error("healthy nodes received no requests")
	}
	if s.Errors[errRefused.Error()] != failed {
		t.Errorf("error messages: %v, want %d refusals", s.Errors, failed)
	}
}

func TestRouterProbeAndRecovery(t *testing.T) {
	rt := newRouter(3, []int{0, 1, 2}, []int{0, 1, 2}, false, time.Second)
	a, b := rt.newCursor(0), rt.newCursor(1)

	rt.report(1, false, false, 0) // node 1 fails at t=0
	for i := 0; i < 6; i++ {
		if node, _ := rt.pick(&a, Get, 500*time.Millisecond); node == 1 {
			t.Fatal("failed node picked before the retry interval")
		}
	}

	// After the interval exactly one request probes the node.
	probes := 0
	for i := 0; i < 6; i++ {
		for _, c := range []*cursor{&a, &b} {
			if node, probe := rt.pick(c, Get, 2*time.Second); node == 1 {
				if !probe {
					t.Fatal("request to a failed node not marked as probe")
				}
				probes++
			}
		}
	}
	if probes != 1 {
		t.Fatalf("%d probes in flight, want 1", probes)
	}

	rt.report(1, true, true, 2*time.Second) // the probe succeeds
	picked := false
	for i := 0; i < 3; i++ {
		if node, probe := rt.pick(&a, Get, 2*time.Second); node == 1 {
			picked = !probe
		}
	}
	if !picked {
		t.Error("recovered node did not return to the rotation")
	}
}

func TestRouterAllFailedStillSends(t *testing.T) {
	rt := newRouter(3, []int{1, 2}, []int{0}, false, time.Second)
	c := rt.newCursor(0)
	rt.report(1, false, false, 0)
	rt.report(2, false, false, 0)
	node, probe := rt.pick(&c, Get, time.Millisecond)
	if (node != 1 && node != 2) || probe {
		t.Errorf("pick = %d, %v; want a read node, not a probe", node, probe)
	}
	if node, _ := rt.pick(&c, Put, time.Millisecond); node != 0 {
		t.Errorf("write went to node %d, want 0", node)
	}
}

func TestTimelineBinsByCompletion(t *testing.T) {
	res := &Result{
		Config: Config{Duration: 3 * time.Second, Warmup: time.Second},
		Start:  time.Unix(1000, 0),
		Nodes:  3,
		recs:   []*recorder{{}},
	}
	add := func(start, lat time.Duration, node int, op Op, st Status) {
		res.recs[0].add(newSample(start, lat, 0, node, op, st))
	}
	add(100*time.Millisecond, 10*time.Millisecond, 0, Get, OK)   // second 0 (warm-up)
	add(900*time.Millisecond, 200*time.Millisecond, 1, Put, OK)  // starts in 0, completes in 1
	add(1500*time.Millisecond, 20*time.Millisecond, 1, Get, OK)  // second 1
	add(2100*time.Millisecond, 50*time.Millisecond, 2, Get, Failed)
	add(2200*time.Millisecond, 500*time.Millisecond, 2, Put, Timeout)

	tl := res.Timeline()
	if len(tl) != 3 {
		t.Fatalf("%d seconds, want 3", len(tl))
	}
	if !tl[0].Warmup || tl[1].Warmup {
		t.Error("warm-up flag wrong")
	}
	if tl[0].All.Get.OK != 1 || tl[1].All.Put.OK != 1 || tl[1].All.Get.OK != 1 {
		t.Errorf("completions binned wrong: %+v %+v", tl[0].All, tl[1].All)
	}
	if tl[1].Nodes[1].Put.P50Ms != 200 {
		t.Errorf("p50 of the PUT on n2 = %v ms, want 200", tl[1].Nodes[1].Put.P50Ms)
	}
	if tl[2].All.Get.Failed != 1 || tl[2].All.Put.Timeouts != 1 || tl[2].Nodes[2].Get.Failed != 1 {
		t.Errorf("errors binned wrong: %+v", tl[2])
	}
	if tl[1].UnixMs != 1001_000 {
		t.Errorf("unix time of second 1 = %d", tl[1].UnixMs)
	}

	// The window is [1 s, 3 s): the first GET is left out.
	s := res.Summarize()
	if s.All.OK != 2 || s.All.Failed != 1 || s.All.Timeouts != 1 || s.Throughput != 1 {
		t.Errorf("summary: ok=%d failed=%d timeouts=%d throughput=%v", s.All.OK, s.All.Failed, s.All.Timeouts, s.Throughput)
	}
}

func TestLiveReportsEverySecond(t *testing.T) {
	store := newFakeStore()
	cfg := testConfig()
	cfg.Duration, cfg.Warmup = 2300*time.Millisecond, time.Second
	var (
		mu  sync.Mutex
		ivs []Interval
	)
	cfg.Live = func(iv Interval) {
		mu.Lock()
		ivs = append(ivs, iv)
		mu.Unlock()
	}
	res, err := Run(context.Background(), store, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(ivs) != 2 {
		t.Fatalf("%d live intervals, want 2", len(ivs))
	}
	total := 0
	for _, iv := range ivs {
		if iv.All.Get.OK == 0 {
			t.Error("live interval without requests")
		}
		total += iv.All.Get.OK + iv.All.Put.OK
	}
	all := 0
	for _, rec := range res.recs {
		all += rec.n
	}
	if total > all {
		t.Errorf("live intervals report %d requests, run recorded %d", total, all)
	}
	if !ivs[0].Warmup || ivs[1].Warmup {
		t.Errorf("warm-up flags: %v %v", ivs[0].Warmup, ivs[1].Warmup)
	}
}

func TestRunInterrupted(t *testing.T) {
	store := newFakeStore()
	cfg := testConfig()
	cfg.Duration, cfg.Warmup = 10*time.Second, 50*time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res, err := Run(ctx, store, cfg)
	if err == nil || res == nil {
		t.Fatalf("res=%v err=%v, want a partial result and the context error", res, err)
	}
	s := res.Summarize()
	if s.DurationS > 1 || s.All.OK == 0 {
		t.Errorf("partial summary: duration=%vs ok=%d", s.DurationS, s.All.OK)
	}
}
