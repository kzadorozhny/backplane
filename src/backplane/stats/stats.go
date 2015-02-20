package stats

import (
	"net/http"
	"sync/atomic"

	"github.com/golang/glog"
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
	RateLimiter *EMARateLimiter
	stats       Counters
}

func (s *CountersCollectingHandler) GetCounters() Counters {
	return s.stats.atomicCopy()
}

func (s *CountersCollectingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	glog.V(3).Infof("handler %s in", req.RequestURI)
	if s.RateLimiter != nil {
		if !s.RateLimiter.Accepted() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}
	s.stats.in()
	s.Handler.ServeHTTP(w, req)
	s.stats.out()
	glog.V(3).Infof("handler %s out", req.RequestURI)
}

type CountersCollectingRoundTripper struct {
	http.RoundTripper
	stats Counters
}

func (s *CountersCollectingRoundTripper) GetCounters() Counters {
	return s.stats.atomicCopy()
}

func (s *CountersCollectingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	s.stats.in()
	resp, err := s.RoundTripper.RoundTrip(r)
	s.stats.out()
	return resp, err
}
