package stats

import (
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/apesternikov/backplane/src/context"

	"golang.org/x/net/trace"
)

/*
Note on stats:
I'm not planning to implement any comprehensive stats here as long as external tools like graphite and influxdb
are doing decent job.
*/

type Counting interface {
	GetCounters() Counters
}

type Counters struct {
	CurActiveSessions, MaxActiveSessions int64
	TotalSessions                        int64
}

// return values from stats without locking.
// information could be inconsistent due to race conditions so it sould be used for informational purposes only
func (s *Counters) atomicCopy() Counters {
	return Counters{
		CurActiveSessions: atomic.LoadInt64(&s.CurActiveSessions),
		MaxActiveSessions: atomic.LoadInt64(&s.MaxActiveSessions),
		TotalSessions:     atomic.LoadInt64(&s.TotalSessions),
	}
}

//should be called on the way in
func (s *Counters) in() {
	atomic.AddInt64(&s.TotalSessions, 1)
	as := atomic.AddInt64(&s.CurActiveSessions, 1)
	for {
		maxas := atomic.LoadInt64(&s.MaxActiveSessions)
		if as > maxas {
			if !atomic.CompareAndSwapInt64(&s.MaxActiveSessions, maxas, as) {
				continue
			}
		}
		break
	}
}

func (s *Counters) out() {
	atomic.AddInt64(&s.CurActiveSessions, -1)
}

type CountersCollectingHandler struct {
	Handler     http.Handler
	RateLimiter RateLimiter
	Limiter     Limiter
	//TraceFamily string
	stats Counters
}

func (s *CountersCollectingHandler) GetCounters() Counters {
	return s.stats.atomicCopy()
}

func (s *CountersCollectingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if s.RateLimiter != nil {
		if !s.RateLimiter.Accepted() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}
	if s.Limiter != nil {
		ctx := context.GetRequestContext(req)
		s.Limiter.Acquire(ctx.Tr)
		defer s.Limiter.Release(ctx.Tr)
	}
	s.stats.in()
	s.Handler.ServeHTTP(w, req)
	s.stats.out()
}

type CountersCollectingRoundTripper struct {
	http.RoundTripper
	RateLimiter RateLimiter
	Limiter     Limiter
	TraceFamily string
	stats       Counters
}

func (s *CountersCollectingRoundTripper) GetCounters() Counters {
	return s.stats.atomicCopy()
}

var RateLimited = errors.New("Rate Limited")

func (s *CountersCollectingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	tr := trace.New(s.TraceFamily, r.RequestURI)
	tr.LazyPrintf("Request: %#v", r)
	defer tr.Finish()
	if s.RateLimiter != nil {
		if !s.RateLimiter.Accepted() {
			tr.LazyPrintf("Rate limited")
			tr.SetError()
			return nil, RateLimited
		}
	}
	s.stats.in()
	resp, err := s.RoundTripper.RoundTrip(r)
	s.stats.out()
	if err != nil {
		tr.LazyPrintf("Error in roundtripper: %s", err)
		tr.SetError()
	}
	tr.LazyPrintf("Response: %v", resp)
	return resp, err
}

func (s *CountersCollectingRoundTripper) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	tr, ok := s.RoundTripper.(canceler)
	if !ok {
		panic(fmt.Errorf("net/http: Client Transport of type %T doesn't support CancelRequest; Timeout not supported", s.RoundTripper))
	}
	tr.CancelRequest(req)
}
