package kv

import (
	"context"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
)

// etcdStore talks to etcd over gRPC. Every node accepts reads and writes: a
// follower forwards writes to the leader and, for linearizable reads, asks
// the leader for the current commit index before answering from its own
// state.
//
// Each client object has a single endpoint, so a request goes to the node
// the caller chose and not to the one a client-side balancer would pick.
type etcdStore struct {
	conns   [][]*clientv3.Client // [node][connection]
	next    []atomic.Uint32      // round-robin over the connections of a node
	getOpts []clientv3.OpOption
	nodes   []int
}

func openEtcd(o Options) (*etcdStore, error) {
	conns := o.Conns
	if conns < 1 {
		conns = 1
	}
	s := &etcdStore{
		conns: make([][]*clientv3.Client, len(o.Addrs)),
		next:  make([]atomic.Uint32, len(o.Addrs)),
		nodes: seq(len(o.Addrs)),
	}
	if o.Serializable {
		s.getOpts = []clientv3.OpOption{clientv3.WithSerializable()}
	}
	for i, addr := range o.Addrs {
		for c := 0; c < conns; c++ {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   []string{withPort(addr, "2379")},
				DialTimeout: o.Timeout,
				// One attempt per request: the library default retries up
				// to 100 times inside a single call, which would hide
				// failures from the samples.
				MaxUnaryRetries: 1,
				Logger:          zap.NewNop(),
				DialOptions: []grpc.DialOption{
					// The default reconnect backoff grows to 120 s. A
					// restarted node must be found again within about a
					// second.
					grpc.WithConnectParams(grpc.ConnectParams{
						Backoff: backoff.Config{
							BaseDelay:  100 * time.Millisecond,
							Multiplier: 1.6,
							Jitter:     0.2,
							MaxDelay:   time.Second,
						},
						MinConnectTimeout: time.Second,
					}),
				},
			})
			if err != nil {
				s.Close()
				return nil, err
			}
			s.conns[i] = append(s.conns[i], cli)
		}
	}
	return s, nil
}

func (s *etcdStore) Nodes() int        { return len(s.conns) }
func (s *etcdStore) ReadNodes() []int  { return s.nodes }
func (s *etcdStore) WriteNodes() []int { return s.nodes }

func (s *etcdStore) client(node int) *clientv3.Client {
	c := s.conns[node]
	if len(c) == 1 {
		return c[0]
	}
	return c[int(s.next[node].Add(1))%len(c)]
}

func (s *etcdStore) Put(ctx context.Context, node int, key string, value []byte) (int, error) {
	_, err := s.client(node).Put(ctx, key, string(value))
	return node, err
}

func (s *etcdStore) Get(ctx context.Context, node int, key string) ([]byte, int, error) {
	resp, err := s.client(node).Get(ctx, key, s.getOpts...)
	if err != nil || len(resp.Kvs) == 0 {
		return nil, node, err
	}
	return resp.Kvs[0].Value, node, nil
}

func (s *etcdStore) Close() error {
	var first error
	for _, node := range s.conns {
		for _, c := range node {
			if err := c.Close(); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}
