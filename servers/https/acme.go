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
	"log/slog"
	"time"

	"github.com/caddyserver/certmagic"
)

type challenge string

const (
	HTTP01    challenge = "http-01"
	TLSAlpn01 challenge = "tlsalpn-01"
)

func IssueCertificates(cacheDir, email, challengeType string, domains []string, useProduction bool, altHTTPPort, altTLSAlpnPort int, log *slog.Logger) (*tls.Config, error) {
	zl := newZap(log)

	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(c certmagic.Certificate) (*certmagic.Config, error) {
			return &certmagic.Config{
				RenewalWindowRatio: 0,
				MustStaple:         false,
				OCSP:               certmagic.OCSPConfig{},
				Storage:            &certmagic.FileStorage{Path: cacheDir},
				Logger:             zl,
			}, nil
		},
		OCSPCheckInterval:  0,
		RenewCheckInterval: 0,
		Capacity:           0,
	})

	cfg := certmagic.New(cache, certmagic.Config{
		RenewalWindowRatio: 0,
		MustStaple:         false,
		OCSP:               certmagic.OCSPConfig{},
		Storage:            &certmagic.FileStorage{Path: cacheDir},
		Logger:             zl,
	})

	myAcme := certmagic.NewACMEIssuer(cfg, certmagic.ACMEIssuer{
		CA:                      certmagic.LetsEncryptProductionCA,
		TestCA:                  certmagic.LetsEncryptStagingCA,
		Email:                   email,
		Agreed:                  true,
		DisableHTTPChallenge:    false,
		DisableTLSALPNChallenge: false,
		ListenHost:              "0.0.0.0",
		AltHTTPPort:             altHTTPPort,
		AltTLSALPNPort:          altTLSAlpnPort,
		CertObtainTimeout:       time.Second * 240,
		PreferredChains:         certmagic.ChainPreference{},
		Logger:                  zl,
	})

	if !useProduction {
		myAcme.CA = certmagic.LetsEncryptStagingCA
	}

	switch challenge(challengeType) {
	case HTTP01:
		myAcme.DisableTLSALPNChallenge = true
	case TLSAlpn01:
		myAcme.DisableHTTPChallenge = true
	default:
		// default - http
		myAcme.DisableTLSALPNChallenge = true
	}

	cfg.Issuers = append(cfg.Issuers, myAcme)

	for i := 0; i < len(domains); i++ {
		err := cfg.ObtainCertAsync(context.Background(), domains[i])
		if err != nil {
			return nil, err
		}
	}

	err := cfg.ManageSync(context.Background(), domains)
	if err != nil {
		return nil, err
	}

	return cfg.TLSConfig(), nil
}
