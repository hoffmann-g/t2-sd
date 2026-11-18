// Package kv is the key-value interface used by the load tools, with one thin
// adapter per system under test. The adapters only translate Put and Get to
// the client library of each system: the choice of operations, keys and nodes
// stays in the caller, so both systems receive the same workload.
package kv

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Store is a replicated key-value store with a fixed set of nodes, numbered
// from 0. Every request names the node the client sends it to.
type Store interface {
	// Nodes returns the number of nodes.
	Nodes() int

	// ReadNodes and WriteNodes list the nodes a client that knows the whole
	// cluster uses for each operation.
	ReadNodes() []int
	WriteNodes() []int

	// Put writes value under key through node. It returns the node that
	// received the request, which differs from node when the system only
	// accepts writes on one node. Put does not retain value.
	Put(ctx context.Context, node int, key string, value []byte) (served int, err error)

	// Get reads key from node. A nil value with a nil error means the key
	// does not exist on that node.
	Get(ctx context.Context, node int, key string) (value []byte, served int, err error)

	Close() error
}

// Options selects and configures an adapter.
type Options struct {
	System  string        // "etcd" or "redis"
	Addrs   []string      // host or host:port of each node on the client network
	Timeout time.Duration // per-request timeout; also bounds connection attempts
	Clients int           // concurrent clients, used to size connection pools

	// etcd
	Conns        int  // gRPC connections per node (default 1)
	Serializable bool // serializable reads instead of linearizable ones

	// redis
	Primary  int    // index of the node that accepts writes
	ReadFrom string // "replicas" (default), "all" or "primary"
}

// Open connects to the system named in o.
func Open(o Options) (Store, error) {
	if len(o.Addrs) == 0 {
		return nil, fmt.Errorf("kv: no node addresses")
	}
	if o.Timeout <= 0 {
		o.Timeout = 2 * time.Second
	}
	if o.Clients < 1 {
		o.Clients = 1
	}
	switch o.System {
	case "etcd":
		return openEtcd(o)
	case "redis":
		return openRedis(o)
	default:
		return nil, fmt.Errorf("kv: unknown system %q (want etcd or redis)", o.System)
	}
}

// withPort appends the default port when addr has none.
func withPort(addr, port string) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return net.JoinHostPort(addr, port)
}

func seq(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}
