//Provides facility to limit simultaneous operations, similar to semaphore

package stats

import "golang.org/x/net/trace"

type Limiter interface {
	// Acquire allocation from the limiter. this call must be accompanied by Release(tr)
	Acquire(tr trace.Trace)
	// Release allocation
	Release(tr trace.Trace)
	// Get current number of allocations acquired.
	Size() int
	// Get configured limit. 0 is unlimited
	Limit() int
}

// Create a new limiter with n allowed items
func NewLimiter(n int) Limiter {
	switch n {
	case 0:
		return unlimitedLimiter{}
	default:
		return &semaLimiter{make(chan struct{}, n)}
	}
}

type unlimitedLimiter struct {
}

func (l unlimitedLimiter) Acquire(tr trace.Trace) {}
func (l unlimitedLimiter) Release(tr trace.Trace) {}
func (l unlimitedLimiter) Size() int              { return 0 }
func (l unlimitedLimiter) Limit() int             { return 0 }

type semaLimiter struct {
	sem chan struct{}
}

func (l *semaLimiter) Acquire(tr trace.Trace) {
	if tr != nil {
		tr.LazyPrintf("Acquiring semalimiter out of %d", l.Limit())
	}
	l.sem <- struct{}{}
	if tr != nil {
		tr.LazyPrintf("semalimiter acquired")
	}
}
func (l *semaLimiter) Release(tr trace.Trace) {
	if tr != nil {
		tr.LazyPrintf("releasing semalimiter")
	}
	<-l.sem
	if tr != nil {
		tr.LazyPrintf("semalimiter released")
	}
}

// Currently used
func (l *semaLimiter) Size() int { return len(l.sem) }

// max configured limit
func (l *semaLimiter) Limit() int { return cap(l.sem) }

//TODO: timed acquire
