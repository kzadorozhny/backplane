package stats

import (
	"sync/atomic"
	"time"
)

type RateLimiter interface {
	// Accepted checks if query at this moment should be accepted or rejected.
	// If accepted, the internal state of the limiter is updated to reflect the accepted counter/current QPS
	Accepted() bool

	TargetQPS() int64

	// MaxQPS returns max achieved EMA(QPS) since the rate limiter created
	MaxQPS() int64

	// TotalAcceptedCount returs total number of accepted queries over the
	// limiter lifetime
	TotalAcceptedCount() int64

	// TotalRejectedCount returs total number of rejected queries over the
	// limiter lifetime
	TotalRejectedCount() int64

	// CurrentQPS returns current EMA of QPS
	CurrentQPS() int64

	LastPacket() time.Time
}

//NewRateLimiter constructs a new EMA rate limiter with specified EMA cutoff
func NewRateLimiter(maxQPS float64) RateLimiter {
	if maxQPS < 0.1 {
		maxQPS = 1000000
	}
	return &emaRateLimiter{
		timeOfLastRequest: time.Now().UnixNano(),
		avgWaitingNs:      1000000000000,
		minWaitingNs:      1000000000000,
		targetQPS:         maxQPS,
		targetWaitingNs:   int64(nanosInSeconds / maxQPS),
	}
}

const wq = 0.05
const nanosInSeconds = float64(time.Second)
const iNanosInSeconds = int64(time.Second)

// emaRateLimiter is a rate limiter used EMA of momentary rate.
// The query is rejected if EMA of the rate would go above the configured
// limit if the query is accepted
type emaRateLimiter struct {
	timeOfLastRequest                           int64
	avgWaitingNs                                int64
	minWaitingNs                                int64
	targetQPS                                   float64 //final
	targetWaitingNs                             int64   //final
	requestThrottledCount, requestAcceptedCount int64
}

/*
Congestion control: on every request accepted we compute
the average QPS using an exponentially weighted moving average: avgQPS =
(1-WQ)avgQPS + WQ*QPSinstantaneous where WQ is a weight and
QPSinstantaneous is calculated as 1000/(time since last pkt sent in ms)
our goal is to keep request rate as close as possible to target QPS. we
maintain the current cutoff request weight using following rules: if
avgQPS is lower than target qps: increase cutoff if request weight is
less than target req weight we drop the request
*/

// Accepted checks if query at this moment should be accepted or rejected.
// If accepted, the EMA rate limiter updates its current EMA
func (e *emaRateLimiter) Accepted() bool {
	now := time.Now().UnixNano()
	instWaiting := now - atomic.LoadInt64(&e.timeOfLastRequest)
	for {
		avgWaitingNs := atomic.LoadInt64(&e.avgWaitingNs)
		newavgWaitingNs := int64((1.-wq)*float64(avgWaitingNs) + wq*float64(instWaiting))
		// glog.V(3).Infof("avgWaitingNs %d newavgWaitingNs %d", avgWaitingNs, newavgWaitingNs)
		if newavgWaitingNs < e.targetWaitingNs {
			atomic.AddInt64(&e.requestThrottledCount, 1)
			return false
		}
		// if(pendingRequests.size()>maxPendingQueueLength) {
		// pendingTooLongDiscarded.incrementAndGet();
		// return false;
		// }
		atomic.StoreInt64(&e.timeOfLastRequest, now)
		newavgWaitingNs2 := newavgWaitingNs
		if !atomic.CompareAndSwapInt64(&e.avgWaitingNs, avgWaitingNs, newavgWaitingNs) {
			continue
		}
		if newavgWaitingNs2 < e.minWaitingNs {
			e.minWaitingNs = newavgWaitingNs2
		}
		atomic.AddInt64(&e.requestAcceptedCount, 1)
		break

	}
	return true
}

// TargetQPS returns configured EMA cutoff rate
func (e *emaRateLimiter) TargetQPS() int64 {
	return iNanosInSeconds / e.targetWaitingNs
}

// MaxQPS returns max achieved EMA(QPS) since the rate limiter created
func (e *emaRateLimiter) MaxQPS() int64 {
	return iNanosInSeconds / e.minWaitingNs
}

// TotalAcceptedCount returs total number of accepted queries over the
// limiter lifetime
func (e *emaRateLimiter) TotalAcceptedCount() int64 {
	return atomic.LoadInt64(&e.requestAcceptedCount)
}

// TotalRejectedCount returs total number of rejected queries over the
// limiter lifetime
func (e *emaRateLimiter) TotalRejectedCount() int64 {
	return atomic.LoadInt64(&e.requestThrottledCount)
}

// CurrentQPS returns current EMA of QPS
func (e *emaRateLimiter) CurrentQPS() int64 {
	return iNanosInSeconds / atomic.LoadInt64(&e.avgWaitingNs)
}

func (e *emaRateLimiter) LastPacket() time.Time {
	return time.Unix(0, atomic.LoadInt64(&e.timeOfLastRequest))
}
