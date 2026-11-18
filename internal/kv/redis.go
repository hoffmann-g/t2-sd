package kv

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

// redisStore talks to one Redis primary with asynchronous replicas. Writes
// are only accepted by the primary, so Put sends every write to it, whatever
// node the caller named. The primary answers a write before the replicas
// receive it. Reads go to the node the caller named and return whatever that
// node has applied so far.
type redisStore struct {
	clients []*redis.Client
	primary int
	read    []int
	write   []int
}

func openRedis(o Options) (*redisStore, error) {
	if o.Primary < 0 || o.Primary >= len(o.Addrs) {
		return nil, fmt.Errorf("kv: primary %d out of range", o.Primary)
	}
	s := &redisStore{primary: o.Primary, write: []int{o.Primary}}
	switch o.ReadFrom {
	case "", "replicas":
		for i := range o.Addrs {
			if i != o.Primary {
				s.read = append(s.read, i)
			}
		}
	case "all":
		s.read = seq(len(o.Addrs))
	case "primary":
		s.read = []int{o.Primary}
	default:
		return nil, fmt.Errorf("kv: unknown read target %q (want replicas, all or primary)", o.ReadFrom)
	}
	if len(s.read) == 0 {
		return nil, errors.New("kv: no node to read from")
	}
	for _, addr := range o.Addrs {
		s.clients = append(s.clients, redis.NewClient(&redis.Options{
			Addr: withPort(addr, "6379"),
			// One connection per concurrent client: a request never waits
			// for a free connection.
			PoolSize:              o.Clients,
			DialTimeout:           o.Timeout,
			ReadTimeout:           o.Timeout,
			WriteTimeout:          o.Timeout,
			ContextTimeoutEnabled: true,
			// One attempt per request: the library default retries 3 times
			// inside a single call, which would hide failures from the
			// samples.
			MaxRetries:      -1,
			DisableIdentity: true,
			MaintNotificationsConfig: &maintnotifications.Config{
				Mode: maintnotifications.ModeDisabled,
			},
		}))
	}
	return s, nil
}

func (s *redisStore) Nodes() int        { return len(s.clients) }
func (s *redisStore) ReadNodes() []int  { return s.read }
func (s *redisStore) WriteNodes() []int { return s.write }

func (s *redisStore) Put(ctx context.Context, _ int, key string, value []byte) (int, error) {
	return s.primary, s.clients[s.primary].Set(ctx, key, value, 0).Err()
}

func (s *redisStore) Get(ctx context.Context, node int, key string) ([]byte, int, error) {
	v, err := s.clients[node].Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, node, nil
	}
	if err != nil {
		return nil, node, err
	}
	return v, node, nil
}

func (s *redisStore) Close() error {
	var first error
	for _, c := range s.clients {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
