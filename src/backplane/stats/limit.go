//Provides facility to limit simultaneous operations, similar to semaphore

package stats

type Limiter struct {
	sem chan struct{}
}

func NewLimiter(n int) *Limiter {
	return &Limiter{make(chan struct{}, n)}
}

func (l *Limiter) Acquire()  { l.sem <- struct{}{} }
func (l *Limiter) Release()  { <-l.sem }
func (l *Limiter) Size() int { return len(l.sem) }

//TODO: timed acquire
