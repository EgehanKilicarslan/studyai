package testutil

import (
	"net/http/httptest"
)

// StreamRecorder wraps httptest.ResponseRecorder for streaming responses
type StreamRecorder struct {
	*httptest.ResponseRecorder
	closeNotifyChan chan bool
}

// NewStreamRecorder creates a new StreamRecorder
func NewStreamRecorder() *StreamRecorder {
	return &StreamRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotifyChan:  make(chan bool, 1),
	}
}

// CloseNotify returns a channel for close notifications
func (w *StreamRecorder) CloseNotify() <-chan bool {
	return w.closeNotifyChan
}
