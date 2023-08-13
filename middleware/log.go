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

package middleware

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	_ io.ReadCloser       = (*wrapper)(nil)
	_ http.ResponseWriter = (*wrapper)(nil)
)

var ErrHijackerNotSupported = errors.New("http.Hijacker interface is not supported")

const requestIDCtx = "request_id"

type wrapper struct {
	io.ReadCloser
	read  int
	write int

	w    http.ResponseWriter
	code int
	data []byte
}

func (w *wrapper) Read(b []byte) (int, error) {
	n, err := w.ReadCloser.Read(b)
	w.read += n
	return n, err
}

func (w *wrapper) WriteHeader(code int) {
	w.code = code
	w.w.WriteHeader(code)
}

func (w *wrapper) Header() http.Header {
	return w.w.Header()
}

func (w *wrapper) Write(b []byte) (int, error) {
	n, err := w.w.Write(b)
	w.write += n
	return n, err
}

func (w *wrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.w.(http.Hijacker); ok {
		return hj.Hijack()
	}

	return nil, nil, ErrHijackerNotSupported
}

func (w *wrapper) Flush() {
	if fl, ok := w.w.(http.Flusher); ok {
		fl.Flush()
	}
}

func (w *wrapper) Close() error {
	return w.ReadCloser.Close()
}

func (w *wrapper) reset() {
	w.code = 0
	w.read = 0
	w.write = 0
	w.w = nil
	w.data = nil
	w.ReadCloser = nil
}

type lm struct {
	pool sync.Pool
	log  *slog.Logger
}

func NewLogMiddleware(next http.Handler, log *slog.Logger) http.Handler {
	l := &lm{
		log: log,
		pool: sync.Pool{
			New: func() interface{} {
				return &wrapper{}
			},
		},
	}

	return l.Log(next)
}

func (l *lm) Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := r.URL.Path

		requestID := uuid.NewString()
		w.Header().Set("X-Request-ID", requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDCtx, requestID))

		bw := l.getW(w)
		defer l.putW(bw)

		r2 := *r
		if r2.Body != nil {
			bw.ReadCloser = r2.Body
			r2.Body = bw
		}

		next.ServeHTTP(bw, &r2)

		end := time.Now()
		latency := end.Sub(start)

		ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
		if err != nil {
			ip = r.RemoteAddr
		}

		attributes := []slog.Attr{
			slog.Int("status", bw.code),
			slog.String("method", r.Method),
			slog.String("path", path),
			slog.String("ip", ip),
			slog.Duration("latency", latency),
			slog.String("user-agent", r.UserAgent()),
			slog.Time("time", end),
			slog.String("request-id", requestID),
		}

		switch {
		case bw.code >= http.StatusBadRequest && bw.code < http.StatusInternalServerError:
			l.log.LogAttrs(context.Background(), slog.LevelWarn, "Incoming request", attributes...)
		case bw.code >= http.StatusInternalServerError:
			l.log.LogAttrs(context.Background(), slog.LevelError, "Incoming request", attributes...)
		default:
			l.log.LogAttrs(context.Background(), slog.LevelInfo, "Incoming request", attributes...)
		}
	})
}

func (l *lm) getW(w http.ResponseWriter) *wrapper {
	wr := l.pool.Get().(*wrapper)
	wr.w = w
	return wr
}

func (l *lm) putW(w *wrapper) {
	w.reset()
	l.pool.Put(w)
}

// GetRequestID returns the request identifier
func GetRequestID(r *http.Request) string {
	requestID, ok := r.Context().Value(requestIDCtx).(string)
	if !ok {
		return ""
	}
	return requestID
}
