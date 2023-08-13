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

package config

import (
	"strings"

	"github.com/roadrunner-server/errors"

	"github.com/rumorshub/http/servers/https"
)

type Config struct {
	// Host and port to handle as http server.
	Address string `mapstructure:"address" json:"address,omitempty" bson:"address,omitempty"`

	// List of the middleware names (order will be preserved).
	Middleware []string `mapstructure:"middleware" json:"middleware,omitempty" bson:"middleware,omitempty"`

	// MaxRequestSize specified max size for payload body in megabytes, default: 100Mb.
	MaxRequestSize uint64 `mapstructure:"max_request_size" json:"max_request_size,omitempty" bson:"max_request_size,omitempty"`

	// SSL defines https server options.
	SSL *https.SSLConfig `mapstructure:"ssl" json:"ssl,omitempty" bson:"ssl,omitempty"`

	// HTTP2 configuration
	HTTP2 *https.HTTP2Config `mapstructure:"http2" json:"http2,omitempty" bson:"http2,omitempty"`
}

func (c *Config) EnableHTTP() bool {
	return c.Address != ""
}

func (c *Config) EnableTLS() bool {
	if c.SSL == nil {
		return false
	}
	if c.SSL.Acme != nil {
		return true
	}
	return c.SSL.Key != "" || c.SSL.Cert != ""
}

func (c *Config) InitDefaults() error {
	if c.MaxRequestSize == 0 {
		c.MaxRequestSize = 100 // 100Mb
	}

	if c.HTTP2 != nil {
		err := c.HTTP2.InitDefaults()
		if err != nil {
			return err
		}
	}

	if c.SSL != nil {
		err := c.SSL.InitDefaults()
		if err != nil {
			return err
		}
	}

	return c.Valid()
}

func (c *Config) Valid() error {
	const op = errors.Op("validation")

	if !c.EnableHTTP() && !c.EnableTLS() {
		return errors.E(op, errors.Str("unable to run http service, no method has been specified (http, https, http/2)"))
	}

	if c.Address != "" && !strings.Contains(c.Address, ":") {
		return errors.E(op, errors.Str("malformed http server address"))
	}

	if c.EnableTLS() {
		err := c.SSL.Valid()
		if err != nil {
			return errors.E(op, err)
		}
	}

	return nil
}
