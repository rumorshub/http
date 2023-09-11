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

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"time"

	rrErrors "github.com/roadrunner-server/errors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/rumorshub/http/config"
	"github.com/rumorshub/http/middleware"
	"github.com/rumorshub/http/servers/listener"
)

type Server struct {
	log          *slog.Logger
	http         *http.Server
	address      string
	redirect     bool
	redirectPort int
}

func NewHTTPServer(handler http.Handler, cfg *config.Config, errLog *log.Logger, log *slog.Logger) *Server {
	var redirect bool
	var redirectPort int

	if cfg.SSL != nil {
		redirect = cfg.SSL.Redirect
		redirectPort = cfg.SSL.Port
	}

	if cfg.HTTP2 != nil && cfg.HTTP2.H2C {
		return &Server{
			log:          log,
			redirect:     redirect,
			redirectPort: redirectPort,
			address:      cfg.Address,
			http: &http.Server{
				Handler: h2c.NewHandler(handler, &http2.Server{
					MaxConcurrentStreams:         cfg.HTTP2.MaxConcurrentStreams,
					PermitProhibitedCipherSuites: false,
				}),
				ReadTimeout:       time.Minute,
				ReadHeaderTimeout: time.Minute,
				WriteTimeout:      time.Minute,
				ErrorLog:          errLog,
			},
		}
	}
	return &Server{
		log:          log,
		redirect:     redirect,
		redirectPort: redirectPort,
		address:      cfg.Address,
		http: &http.Server{
			ReadHeaderTimeout: time.Minute * 5,
			Handler:           handler,
			ErrorLog:          errLog,
		},
	}
}

func (s *Server) Start(mdwr map[string]middleware.Middleware, order []string) error {
	const op = rrErrors.Op("serveHTTP")

	for i := 0; i < len(order); i++ {
		if m, ok := mdwr[order[i]]; ok {
			s.http.Handler = m.Middleware(s.http.Handler)
		} else {
			s.log.Warn("requested middleware does not exist", "requested", order[i])
		}
	}

	// apply redirect middleware first (if redirect specified)
	if s.redirect {
		s.http.Handler = middleware.Redirect(s.http.Handler, s.redirectPort)
	}

	l, err := listener.CreateListener(s.address)
	if err != nil {
		return rrErrors.E(op, err)
	}

	s.log.Debug("http server was started", "address", s.address)
	err = s.http.Serve(l)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return rrErrors.E(op, err)
	}

	return nil
}

func (s *Server) GetServer() *http.Server {
	return s.http
}

func (s *Server) Stop() {
	err := s.http.Shutdown(context.Background())
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.log.Error("http shutdown", "error", err)
	}
}
