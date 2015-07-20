package stats

import "net/http"

type StatsCollectingResponseWriter struct {
	http.ResponseWriter
	ResponseCode int
	ResponseSize int
}

func (s *StatsCollectingResponseWriter) Write(data []byte) (int, error) {
	sz, err := s.ResponseWriter.Write(data)
	s.ResponseSize += sz
	return sz, err
}
func (s *StatsCollectingResponseWriter) WriteHeader(code int) {
	s.ResponseCode = code
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
