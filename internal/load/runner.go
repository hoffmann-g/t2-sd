package load

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"

	"t2sd/internal/kv"
)

// Config describes one run.
type Config struct {
	Threads   int           // number of clients (N)
	Duration  time.Duration // length of the run, warm-up included
	Warmup    time.Duration // initial period left out of the statistics
	Keys      int           // size of the key space; keys are drawn uniformly
	ValueSize int           // bytes per value
	PutRatio  float64       // fraction of requests that are PUT; the rest are GET
	Timeout   time.Duration // per-request timeout
	Seed      uint64        // seed of the operation and key draws

	// Pinned makes client i send every request through node i mod N.
	// Otherwise requests rotate over the nodes (see router).
	Pinned bool

	// NodeRetry is how long a node is skipped after a failed request, in
	// rotation mode.
	NodeRetry time.Duration

	// ErrorPause is the minimum time between the start of a failed request
	// and the next request of the same client. A request that fails at once
	// (connection refused) would otherwise be repeated in a busy loop.
	ErrorPause time.Duration

	// Live, if not nil, receives the statistics of each elapsed second
	// while the run is in progress.
	Live func(Interval)
}

// Limits imposed by the 16-byte sample.
const (
	maxKeys      = 1 << 16
	minValueSize = stampLen
)

func (c *Config) validate() error {
	switch {
	case c.Threads < 1:
		return errors.New("load: threads must be at least 1")
	case c.Duration <= 0:
		return errors.New("load: duration must be positive")
	case c.Warmup < 0 || c.Warmup >= c.Duration:
		return errors.New("load: warm-up must be shorter than the duration")
	case c.Keys < 1 || c.Keys > maxKeys:
		return fmt.Errorf("load: keys must be between 1 and %d", maxKeys)
	case c.ValueSize < minValueSize:
		return fmt.Errorf("load: value size must be at least %d bytes", minValueSize)
	case c.PutRatio < 0 || c.PutRatio > 1:
		return errors.New("load: put ratio must be between 0 and 1")
	case c.Timeout <= 0 || c.Timeout > 4*time.Second:
		return errors.New("load: timeout must be between 0 and 4s")
	}
	if c.NodeRetry <= 0 {
		c.NodeRetry = time.Second
	}
	return nil
}

// Result holds every sample of a run.
type Result struct {
	Config Config
	Start  time.Time // wall-clock time at which the run started
	Nodes  int

	recs   []*recorder
	errors map[string]int
	cpu    time.Duration // generator CPU time spent inside the measured window
}

// KeyName returns the key with index i.
func KeyName(i int) string { return fmt.Sprintf("k%05d", i) }

// stampLen is the length of the header written at the start of every value:
// "CCCCC:SSSSSSSSSSSS:", the client and the sequence number of the request.
// It identifies the write that produced a value read back later.
const stampLen = 19

// preloadClient is the client number used in the values written by Preload.
const preloadClient = 99999

func stamp(buf []byte, client int, seq uint64) {
	putDec(buf[0:5], uint64(client))
	buf[5] = ':'
	putDec(buf[6:18], seq)
	buf[18] = ':'
}

func putDec(dst []byte, v uint64) {
	for i := len(dst) - 1; i >= 0; i-- {
		dst[i] = byte('0' + v%10)
		v /= 10
	}
}

// ParseStamp extracts the client and the sequence number from a value.
func ParseStamp(value []byte) (client int, seq uint64, ok bool) {
	if len(value) < stampLen || value[5] != ':' || value[18] != ':' {
		return 0, 0, false
	}
	c, ok1 := getDec(value[0:5])
	s, ok2 := getDec(value[6:18])
	return int(c), s, ok1 && ok2
}

func getDec(src []byte) (uint64, bool) {
	var v uint64
	for _, b := range src {
		if b < '0' || b > '9' {
			return 0, false
		}
		v = v*10 + uint64(b-'0')
	}
	return v, true
}

// newValue returns a value buffer filled with letters drawn from rng. The
// first stampLen bytes are overwritten by stamp before each write.
func newValue(size int, rng *rand.Rand) []byte {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	v := make([]byte, size)
	for i := range v {
		v[i] = letters[rng.IntN(len(letters))]
	}
	return v
}

// Preload writes every key once, so that reads find a value from the first
// request on. It uses the same value size as the run.
func Preload(ctx context.Context, store kv.Store, cfg Config) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	const workers = 32
	write := store.WriteNodes()
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		first error
	)
	next := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			value := newValue(cfg.ValueSize, rand.New(rand.NewPCG(cfg.Seed, preloadClient+uint64(w))))
			for k := range next {
				stamp(value, preloadClient, uint64(k))
				rctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
				_, err := store.Put(rctx, write[k%len(write)], KeyName(k), value)
				cancel()
				if err != nil {
					mu.Lock()
					if first == nil {
						first = fmt.Errorf("preload %s: %w", KeyName(k), err)
					}
					mu.Unlock()
				}
			}
		}(w)
	}
	for k := 0; k < cfg.Keys; k++ {
		next <- k
	}
	close(next)
	wg.Wait()
	return first
}

// Run executes the closed loop until cfg.Duration elapses or ctx is
// cancelled. Requests still in flight at that moment are abandoned and not
// recorded.
func Run(ctx context.Context, store kv.Store, cfg Config) (*Result, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if cfg.Pinned && cfg.Threads < store.Nodes() {
		return nil, fmt.Errorf("load: pinned mode needs at least %d threads, one per node", store.Nodes())
	}

	keys := make([]string, cfg.Keys)
	for i := range keys {
		keys[i] = KeyName(i)
	}
	res := &Result{
		Config: cfg,
		Nodes:  store.Nodes(),
		recs:   make([]*recorder, cfg.Threads),
		errors: make(map[string]int),
	}
	for i := range res.recs {
		res.recs[i] = &recorder{}
	}
	rt := newRouter(store.Nodes(), store.ReadNodes(), store.WriteNodes(), cfg.Pinned, cfg.NodeRetry)

	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	var (
		wg     sync.WaitGroup
		errMu  sync.Mutex
		cpuAt  time.Duration // CPU time consumed when the warm-up ended
		cpuOK  bool
		cpuSet = make(chan struct{})
	)
	res.Start = time.Now()
	start := res.Start // carries the monotonic clock reading

	for id := 0; id < cfg.Threads; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			errs := client(runCtx, store, rt, &cfg, keys, start, id, res.recs[id])
			if len(errs) == 0 {
				return
			}
			errMu.Lock()
			for msg, n := range errs {
				res.errors[msg] += n
			}
			errMu.Unlock()
		}(id)
	}

	go func() {
		defer close(cpuSet)
		select {
		case <-time.After(cfg.Warmup):
			cpuAt, cpuOK = cpuTime(), true
		case <-runCtx.Done():
		}
	}()

	liveDone := make(chan struct{})
	go func() {
		defer close(liveDone)
		if cfg.Live == nil {
			return
		}
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		var buf []Sample
		for {
			select {
			case <-tick.C:
				buf = buf[:0]
				for _, r := range res.recs {
					buf = r.unseen(buf)
				}
				now := time.Since(start)
				iv := aggregate(buf, res.Nodes)
				iv.T = now.Seconds()
				iv.UnixMs = start.Add(now).UnixMilli()
				// The interval covers the second that just ended; it is warm-up
				// if it started before the warm-up ended, as in Timeline.
				iv.Warmup = now-time.Second < cfg.Warmup
				cfg.Live(iv)
			case <-runCtx.Done():
				return
			}
		}
	}()

	select {
	case <-time.After(cfg.Duration):
	case <-ctx.Done():
	}
	elapsed := time.Since(start)
	cpuEnd := cpuTime()
	stop()
	wg.Wait()
	<-liveDone
	<-cpuSet
	if cpuOK {
		res.cpu = cpuEnd - cpuAt
	}
	if err := ctx.Err(); err != nil {
		// Interrupted: the result covers the time the run lasted.
		res.Config.Duration = elapsed
		if res.Config.Warmup >= elapsed {
			res.Config.Warmup, res.cpu = 0, 0
		}
		return res, err
	}
	return res, nil
}

// client is the loop of one client. It returns the error messages it saw,
// with their counts.
func client(ctx context.Context, store kv.Store, rt *router, cfg *Config, keys []string, start time.Time, id int, rec *recorder) map[string]int {
	rng := rand.New(rand.NewPCG(cfg.Seed, uint64(id)))
	value := newValue(cfg.ValueSize, rng)
	cur := rt.newCursor(id)
	var errs map[string]int

	for seq := uint64(0); ; seq++ {
		op := Get
		if rng.Float64() < cfg.PutRatio {
			op = Put
		}
		key := rng.IntN(len(keys))

		t0 := time.Since(start)
		if t0 >= cfg.Duration || ctx.Err() != nil {
			return errs
		}
		via, probe := rt.pick(&cur, op, t0)

		rctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		var (
			node int // node that received the request
			err  error
		)
		if op == Put {
			stamp(value, id, seq)
			node, err = store.Put(rctx, via, keys[key], value)
		} else {
			_, node, err = store.Get(rctx, via, keys[key])
		}
		t1 := time.Since(start)
		timedOut := rctx.Err() == context.DeadlineExceeded
		cancel()

		if ctx.Err() != nil {
			// The run ended while the request was in flight.
			rt.report(via, probe, err == nil, t1)
			return errs
		}

		st := OK
		if err != nil {
			st = Failed
			if timedOut {
				st = Timeout
			}
			if errs == nil {
				errs = make(map[string]int)
			}
			if msg := err.Error(); len(errs) < 64 || errs[msg] > 0 {
				errs[msg]++
			} else {
				errs["(other errors)"]++
			}
		}
		rt.report(via, probe, err == nil, t1)
		rec.add(newSample(t0, t1-t0, key, node, op, st))

		if err != nil && cfg.ErrorPause > 0 {
			if wait := cfg.ErrorPause - (t1 - t0); wait > 0 {
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return errs
				}
			}
		}
	}
}

// CPUs is the number of logical CPUs the generator may use.
func CPUs() int { return runtime.GOMAXPROCS(0) }
