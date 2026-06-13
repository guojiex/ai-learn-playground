package web

import (
	"net/http/httptest"
)

// flushRecorder 在 httptest.ResponseRecorder 之上加 Flush, 让 SSE handler 满意.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (*flushRecorder) Flush() {}
