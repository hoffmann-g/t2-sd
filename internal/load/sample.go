// Package load is the closed-loop load generator: N clients that send one
// request at a time, each recording a sample per completed request.
package load

import (
	"math"
	"sync"
	"time"
)

// Op is the kind of a request.
type Op uint8

const (
	Get Op = iota
	Put
)

func (o Op) String() string {
	if o == Put {
		return "PUT"
	}
	return "GET"
}

// Status is the outcome of a request.
type Status uint8

const (
	OK      Status = iota // answered within the timeout
	Timeout               // no answer within the timeout
	Failed                // answered with an error, or could not be sent
)

func (s Status) String() string {
	switch s {
	case OK:
		return "ok"
	case Timeout:
		return "timeout"
	default:
		return "failed"
	}
}

// Sample is one completed request. It takes 16 bytes, so a run with tens of
// millions of requests fits in memory and nothing is written to disk while
// the load is running.
type Sample struct {
	Start int64  // send time, in nanoseconds since the run started
	Lat   uint32 // time from send to completion, in nanoseconds
	Key   uint16 // key index
	Node  uint8  // node that received the request
	flags uint8  // bit 0: op; bits 1-2: status
}

// maxLat is the longest time a Sample can hold (about 4.29 s). Config
// validation keeps the request timeout below it.
const maxLat = time.Duration(math.MaxUint32)

func newSample(start, lat time.Duration, key, node int, op Op, st Status) Sample {
	if lat > maxLat {
		lat = maxLat
	}
	return Sample{
		Start: int64(start),
		Lat:   uint32(lat),
		Key:   uint16(key),
		Node:  uint8(node),
		flags: uint8(op) | uint8(st)<<1,
	}
}

func (s Sample) Op() Op         { return Op(s.flags & 1) }
func (s Sample) Status() Status { return Status(s.flags >> 1 & 3) }

// End is the completion time, in nanoseconds since the run started.
func (s Sample) End() int64 { return s.Start + int64(s.Lat) }

// chunkSize is the number of samples per chunk (256 KiB). Samples are kept
// in fixed chunks so that recording never copies what was already recorded.
const chunkSize = 1 << 14

// recorder holds the samples of one client. The mutex is only contended once
// per second, when the live reporter reads the new samples.
type recorder struct {
	mu     sync.Mutex
	chunks [][]Sample
	n      int
	seen   int // samples already read by the live reporter
}

func (r *recorder) add(s Sample) {
	r.mu.Lock()
	if r.n%chunkSize == 0 {
		r.chunks = append(r.chunks, make([]Sample, 0, chunkSize))
	}
	last := len(r.chunks) - 1
	r.chunks[last] = append(r.chunks[last], s)
	r.n++
	r.mu.Unlock()
}

// unseen appends to dst the samples added since the previous call.
func (r *recorder) unseen(dst []Sample) []Sample {
	r.mu.Lock()
	for i := r.seen; i < r.n; {
		c := r.chunks[i/chunkSize]
		from := i % chunkSize
		dst = append(dst, c[from:]...)
		i += len(c) - from
	}
	r.seen = r.n
	r.mu.Unlock()
	return dst
}

// each calls f for every sample, in recording order. It must only be used
// after the client has stopped.
func (r *recorder) each(f func(i int, s Sample)) {
	i := 0
	for _, c := range r.chunks {
		for _, s := range c {
			f(i, s)
			i++
		}
	}
}
