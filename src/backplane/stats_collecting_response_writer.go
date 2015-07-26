package backplane

import (
	"github.com/apesternikov/backplane/src/context"

	"net/http"
)

//http.ResponseWriter implementation that collects status code and response size, as well as updating
//response headers

type StatsCollectingResponseWriter struct {
	http.ResponseWriter
	ServerName   string
	ResponseCode int
	ResponseSize int
	ctx          *context.RequestContext
}

func (s *StatsCollectingResponseWriter) Write(data []byte) (int, error) {
	if s.ResponseCode == 0 {
		s.WriteHeader(200)
	}
	sz, err := s.ResponseWriter.Write(data)
	s.ResponseSize += sz
	return sz, err
}
func (s *StatsCollectingResponseWriter) WriteHeader(code int) {
	s.ResponseCode = code
	if s.ServerName == "" {
		s.Header().Set("Server", "backplane/0.1")
	} else {
		s.Header().Set("Server", s.ServerName)
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *StatsCollectingResponseWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *StatsCollectingResponseWriter) IsErrorResponse() bool {
	return s.ResponseCode != 0 && s.ResponseCode/100 != 2 && s.ResponseCode/100 != 3
}
