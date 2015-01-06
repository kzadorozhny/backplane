package swim

import "sync/atomic"

type LamportClock struct {
	lamportTimestamp int64
}

// Get epoch (lamport timestamp http://en.wikipedia.org/wiki/Lamport_timestamps of the system)
func (c *LamportClock) GetEpoch() int64 {
	return atomic.LoadInt64(&c.lamportTimestamp)
}

// Increment lamport timestamp. Should be called before any event occuring in the node.
// Returns the new epoch value
func (c *LamportClock) IncrementEpoch() int64 {
	return atomic.AddInt64(&c.lamportTimestamp, 1)
}

// update the lamport timestamp if needed. Should be called with an epoch value received from another node
// Returns the new epoch value
func (c *LamportClock) OnReceivedEpoch(other int64) int64 {
	for {
		cur := atomic.LoadInt64(&c.lamportTimestamp)
		if other < cur {
			return cur
		}
		newval := other + 1
		if atomic.CompareAndSwapInt64(&c.lamportTimestamp, cur, newval) {
			return newval
		}
	}
}
