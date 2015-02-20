package stats

import (
	"sync/atomic"
	"time"

	"github.com/golang/glog"
)

const WQ = 0.05
const nanosInSeconds = float64(time.Second)
const iNanosInSeconds = int64(time.Second)

type EMARateLimiter struct {
	timeOfLastRequest                           int64
	avgWaitingNs                                int64
	minWaitingNs                                int64
	targetQPS                                   float64 //final
	targetWaitingNs                             int64   //final
	requestThrottledCount, requestAcceptedCount int64
}

func NewEMARateLimiter(maxQPS float64) *EMARateLimiter {
	return &EMARateLimiter{
		timeOfLastRequest: time.Now().UnixNano(),
		avgWaitingNs:      1000000000000,
		minWaitingNs:      1000000000000,
		targetQPS:         maxQPS,
		targetWaitingNs:   int64(nanosInSeconds / maxQPS),
	}
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

func (e *EMARateLimiter) Accepted() bool {
	now := time.Now().UnixNano()
	instWaiting := now - e.timeOfLastRequest
	for {
		avgWaitingNs := atomic.LoadInt64(&e.avgWaitingNs)
		newavgWaitingNs := int64((1.-WQ)*float64(avgWaitingNs) + WQ*float64(instWaiting))
		glog.V(3).Infof("avgWaitingNs %d newavgWaitingNs %d", avgWaitingNs, newavgWaitingNs)
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

func (e *EMARateLimiter) TargetQPS() int64 {
	return iNanosInSeconds / e.targetWaitingNs
}

func (e *EMARateLimiter) MaxQPS() int64 {
	return iNanosInSeconds / e.minWaitingNs
}

func (e *EMARateLimiter) TotalAcceptedCount() int64 {
	return atomic.LoadInt64(&e.requestAcceptedCount)
}

func (e *EMARateLimiter) TotalRejectedCount() int64 {
	return atomic.LoadInt64(&e.requestThrottledCount)
}

func (e *EMARateLimiter) CurrentQPS() int64 {
	return iNanosInSeconds / atomic.LoadInt64(&e.avgWaitingNs)
}
