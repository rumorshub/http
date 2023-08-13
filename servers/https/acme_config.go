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

import "github.com/roadrunner-server/errors"

type AcmeConfig struct {
	// directory to save the certificates, le_certs default
	CacheDir string `mapstructure:"cache_dir" json:"cache_dir,omitempty" bson:"cache_dir,omitempty"`

	// User email, mandatory
	Email string `mapstructure:"email" json:"email,omitempty" bson:"email,omitempty"`

	// supported values: http-01, tlsalpn-01
	ChallengeType string `mapstructure:"challenge_type" json:"challenge_type,omitempty" bson:"challenge_type,omitempty"`

	// The alternate port to use for the ACME HTTP challenge
	AltHTTPPort int `mapstructure:"alt_http_port" json:"alt_http_port,omitempty" bson:"alt_http_port,omitempty"`

	// The alternate port to use for the ACME TLS-ALPN
	AltTLSALPNPort int `mapstructure:"alt_tlsalpn_port" json:"alt_tlsalpn_port,omitempty" bson:"alt_tlsalpn_port,omitempty"`

	// Use LE production endpoint or staging
	UseProductionEndpoint bool `mapstructure:"use_production_endpoint" json:"use_production_endpoint,omitempty" bson:"use_production_endpoint,omitempty"`

	// Domains to obtain certificates
	Domains []string `mapstructure:"domains" json:"domains,omitempty" bson:"domains,omitempty"`
}

func (ac *AcmeConfig) InitDefaults() error {
	if ac.CacheDir == "" {
		ac.CacheDir = "cache_dir"
	}

	if ac.Email == "" {
		return errors.Str("email could not be empty")
	}

	if len(ac.Domains) == 0 {
		return errors.Str("should be at least 1 domain")
	}

	if ac.ChallengeType == "" {
		ac.ChallengeType = "http-01"
		if ac.AltHTTPPort == 0 {
			ac.AltHTTPPort = 80
		}
	}

	return nil
}
