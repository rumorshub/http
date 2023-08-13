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

package https

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mholt/acmez"
	rrErrors "github.com/roadrunner-server/errors"
	"golang.org/x/sys/cpu"

	"github.com/rumorshub/http/middleware"
	"github.com/rumorshub/http/servers/listener"
)

type Server struct {
	cfg   *SSLConfig
	log   *slog.Logger
	https *http.Server
}

func NewHTTPSServer(handler http.Handler, cfg *SSLConfig, cfgHTTP2 *HTTP2Config, errLog *log.Logger, logger *slog.Logger) (*Server, error) {
	httpsServer := initTLS(handler, errLog, cfg.Address, cfg.Port)

	if cfg.RootCA != "" {
		pool, err := createCertPool(cfg.RootCA)
		if err != nil {
			return nil, err
		}

		if pool != nil {
			httpsServer.TLSConfig.ClientCAs = pool
			// auth type used only for the CA
			switch cfg.AuthType {
			case NoClientCert:
				httpsServer.TLSConfig.ClientAuth = tls.NoClientCert
			case RequestClientCert:
				httpsServer.TLSConfig.ClientAuth = tls.RequestClientCert
			case RequireAnyClientCert:
				httpsServer.TLSConfig.ClientAuth = tls.RequireAnyClientCert
			case VerifyClientCertIfGiven:
				httpsServer.TLSConfig.ClientAuth = tls.VerifyClientCertIfGiven
			case RequireAndVerifyClientCert:
				httpsServer.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
			default:
				httpsServer.TLSConfig.ClientAuth = tls.NoClientCert
			}
		}
	}

	if cfg.EnableACME() {
		tlsCfg, err := IssueCertificates(
			cfg.Acme.CacheDir,
			cfg.Acme.Email,
			cfg.Acme.ChallengeType,
			cfg.Acme.Domains,
			cfg.Acme.UseProductionEndpoint,
			cfg.Acme.AltHTTPPort,
			cfg.Acme.AltTLSALPNPort,
			logger,
		)

		if err != nil {
			return nil, err
		}

		httpsServer.TLSConfig.GetCertificate = tlsCfg.GetCertificate
		httpsServer.TLSConfig.NextProtos = append(httpsServer.TLSConfig.NextProtos, acmez.ACMETLS1Protocol)
	}

	if cfgHTTP2 != nil && cfgHTTP2.EnableHTTP2() {
		err := initHTTP2(httpsServer, cfgHTTP2.MaxConcurrentStreams)
		if err != nil {
			return nil, err
		}
	}

	return &Server{
		cfg:   cfg,
		log:   logger,
		https: httpsServer,
	}, nil
}

func (s *Server) Start(mdwr map[string]middleware.Middleware, order []string) error {
	const op = rrErrors.Op("serveHTTPS")

	if len(mdwr) > 0 {
		for i := 0; i < len(order); i++ {
			if m, ok := mdwr[order[i]]; ok {
				s.https.Handler = m.Middleware(s.https.Handler)
			} else {
				s.log.Warn("requested middleware does not exist", "requested", order[i])
			}
		}
	}

	l, err := listener.CreateListener(s.cfg.Address)
	if err != nil {
		return rrErrors.E(op, err)
	}

	if s.cfg.EnableACME() {
		s.log.Debug("https(acme) server was started", "address", s.cfg.Address)
		err = s.https.ServeTLS(
			l,
			"",
			"",
		)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return rrErrors.E(op, err)
		}

		return nil
	}

	s.log.Debug("https server was started", "address", s.cfg.Address)
	err = s.https.ServeTLS(
		l,
		s.cfg.Cert,
		s.cfg.Key,
	)

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return rrErrors.E(op, err)
	}

	return nil
}

func (s *Server) GetServer() *http.Server {
	return s.https
}

func (s *Server) Stop() {
	err := s.https.Shutdown(context.Background())
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.log.Error("https shutdown", "error", err)
	}
}

// append RootCA to the https server TLS config
func createCertPool(rootCa string) (*x509.CertPool, error) {
	const op = rrErrors.Op("http_plugin_append_root_ca")
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, nil
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	CA, err := os.ReadFile(rootCa)
	if err != nil {
		return nil, err
	}

	// should append our CA cert
	ok := rootCAs.AppendCertsFromPEM(CA)
	if !ok {
		return nil, rrErrors.E(op, rrErrors.Str("could not append Certs from PEM"))
	}

	return rootCAs, nil
}

// Init https server
func initTLS(handler http.Handler, errLog *log.Logger, addr string, port int) *http.Server {
	var topCipherSuites []uint16
	var defaultCipherSuitesTLS13 []uint16

	hasGCMAsmAMD64 := cpu.X86.HasAES && cpu.X86.HasPCLMULQDQ
	hasGCMAsmARM64 := cpu.ARM64.HasAES && cpu.ARM64.HasPMULL
	// Keep in sync with crypto/aes/cipher_s390x.go.
	hasGCMAsmS390X := cpu.S390X.HasAES && cpu.S390X.HasAESCBC && cpu.S390X.HasAESCTR && (cpu.S390X.HasGHASH || cpu.S390X.HasAESGCM)

	hasGCMAsm := hasGCMAsmAMD64 || hasGCMAsmARM64 || hasGCMAsmS390X

	if hasGCMAsm {
		// If AES-GCM hardware is provided then priorities AES-GCM
		// cipher suites.
		topCipherSuites = []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		}
		defaultCipherSuitesTLS13 = []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
		}
	} else {
		// Without AES-GCM hardware, we put the ChaCha20-Poly1305
		// cipher suites first.
		topCipherSuites = []uint16{
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		}
		defaultCipherSuitesTLS13 = []uint16{
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
		}
	}

	DefaultCipherSuites := make([]uint16, 0, 22)
	DefaultCipherSuites = append(DefaultCipherSuites, topCipherSuites...)
	DefaultCipherSuites = append(DefaultCipherSuites, defaultCipherSuitesTLS13...)

	sslServer := &http.Server{
		Addr:              tlsAddr(addr, true, port),
		Handler:           handler,
		ErrorLog:          errLog,
		ReadHeaderTimeout: time.Minute * 5,
		TLSConfig: &tls.Config{
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
				tls.CurveP384,
				tls.CurveP521,
			},
			CipherSuites: DefaultCipherSuites,
			MinVersion:   tls.VersionTLS12,
		},
	}

	return sslServer
}

// tlsAddr replaces listen or host port with port configured by SSLConfig config.
func tlsAddr(host string, forcePort bool, sslPort int) string {
	// remove current forcePort first
	host = strings.Split(host, ":")[0]

	if forcePort || sslPort != 443 {
		host = fmt.Sprintf("%s:%v", host, sslPort)
	}

	return host
}
