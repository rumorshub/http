// MIT License
//
// Copyright (c) 2023 Spiral Scout
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package http

import "log/slog"

// StdLogAdapter can be passed to the http.Server or any place which required standard logger to redirect output
// to the logger plugin
type StdLogAdapter struct {
	log *slog.Logger
}

// Write io.Writer interface implementation
func (s *StdLogAdapter) Write(p []byte) (n int, err error) {
	s.log.Error("internal server error", "error", string(p))
	return len(p), nil
}

// NewStdAdapter constructs StdLogAdapter
func NewStdAdapter(log *slog.Logger) *StdLogAdapter {
	logAdapter := &StdLogAdapter{
		log: log,
	}

	return logAdapter
}
