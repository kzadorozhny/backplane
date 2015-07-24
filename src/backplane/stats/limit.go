//Provides facility to limit simultaneous operations, similar to semaphore

package stats

type Limiter struct {
	sem chan struct{}
}

// Create a new limiter with n allowed items
func NewLimiter(n int) *Limiter {
	return &Limiter{make(chan struct{}, n)}
}

func (l *Limiter) Acquire() { l.sem <- struct{}{} }
func (l *Limiter) Release() { <-l.sem }

// Currently used
func (l *Limiter) Size() int { return len(l.sem) }

// max configured limit
func (l *Limiter) Limit() int { return cap(l.sem) }

//TODO: timed acquire
