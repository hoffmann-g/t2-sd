package load

import (
	"sync/atomic"
	"time"
)

// router chooses the node of each request.
//
// In rotation mode a client knows every node. Each request goes to the next
// node of the list for its operation. A node whose last request failed is
// skipped for the retry interval; after that, a single request probes it,
// and the node returns to the rotation when a request to it succeeds. If
// every node of the list is being skipped, the next one is used anyway.
//
// In pinned mode client i sends every request through node i mod N and never
// skips it. This mode shows what the clients of each node observe.
type router struct {
	pinned bool
	nodes  int
	read   []int
	write  []int
	retry  time.Duration
	state  []nodeState
}

type nodeState struct {
	until   atomic.Int64 // run time (ns) until which the node is skipped; 0: healthy
	probing atomic.Bool  // a probe request is in flight
}

func newRouter(nodes int, read, write []int, pinned bool, retry time.Duration) *router {
	return &router{
		pinned: pinned,
		nodes:  nodes,
		read:   read,
		write:  write,
		retry:  retry,
		state:  make([]nodeState, nodes),
	}
}

// cursor is the position of one client in the read and write rotations.
type cursor struct {
	home  int
	read  int
	write int
}

func (r *router) newCursor(client int) cursor {
	return cursor{home: client % r.nodes, read: client, write: client}
}

// pick returns the node for the next request of the client that owns c.
// probe reports that the request is the probe of a skipped node.
func (r *router) pick(c *cursor, op Op, now time.Duration) (node int, probe bool) {
	if r.pinned {
		return c.home, false
	}
	list, pos := r.read, &c.read
	if op == Put {
		list, pos = r.write, &c.write
	}
	for range list {
		node = list[*pos%len(list)]
		*pos++
		st := &r.state[node]
		until := st.until.Load()
		if until == 0 {
			return node, false
		}
		if int64(now) >= until && st.probing.CompareAndSwap(false, true) {
			return node, true
		}
	}
	node = list[*pos%len(list)]
	*pos++
	return node, false
}

// report records the outcome of a request sent to node.
func (r *router) report(node int, probe, ok bool, now time.Duration) {
	if r.pinned {
		return
	}
	st := &r.state[node]
	if ok {
		st.until.Store(0)
	} else {
		st.until.Store(int64(now + r.retry))
	}
	if probe {
		st.probing.Store(false)
	}
}
