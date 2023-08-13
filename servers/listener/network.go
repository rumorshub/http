//go:build linux || darwin || freebsd

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

package listener

import (
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/roadrunner-server/tcplisten"
)

const (
	IPV4 string = "tcp4"
	IPV6 string = "tcp6"
)

// CreateListener
//
//   - SO_REUSEPORT. This option allows linear scaling server performance
//     on multi-CPU servers.
//     See https://www.nginx.com/blog/socket-sharding-nginx-release-1-9-1/ for details.
//
//   - TCP_DEFER_ACCEPT. This option expects the server reads from the accepted
//     connection before writing to them.
//
//   - TCP_FASTOPEN. See https://lwn.net/Articles/508865/ for details.
//
// CreateListener crates socket listener based on DSN definition.
func CreateListener(address string) (net.Listener, error) {
	dsn := strings.Split(address, "://")

	switch len(dsn) {
	case 1:
		// assume, that there is no prefix here [127.0.0.1:8000]
		return createTCPListener(dsn[0])
	case 2:
		// we got two part here, first part is the transport, second - address
		// [tcp://127.0.0.1:8000] OR [unix:///path/to/unix.socket] OR [error://path]
		// where error is wrong transport name
		switch dsn[0] {
		case "unix":
			// check of file exist. If exist, unlink
			if fileExists(dsn[1]) {
				err := syscall.Unlink(dsn[1])
				if err != nil {
					return nil, fmt.Errorf("error during the unlink syscall: error %w", err)
				}
			}
			return net.Listen(dsn[0], dsn[1])
		case "tcp":
			return createTCPListener(dsn[1])
			// not an tcp or unix
		default:
			return nil, fmt.Errorf("invalid Protocol ([tcp://]:6001, unix://file.sock), address: %s", address)
		}
		// wrong number of split parts
	default:
		return nil, fmt.Errorf("wrong number of parsed protocol parts, address: %s", address)
	}
}

func createTCPListener(addr string) (net.Listener, error) {
	cfg := tcplisten.Config{
		ReusePort:   true,
		DeferAccept: false,
		FastOpen:    true,
	}

	/*
		Options we may have here:
		1. [::1]:8080 //ipv6
		2. [0:0:..]:8080 //ipv6
		3. 127.0.0.1:8080 //ipv4
		4. :8080 //ipv4
		5. [::]:8080 //ipv6
	*/
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	// consider this is IPv4
	if host == "" {
		return cfg.NewListener(IPV4, addr)
	}

	return cfg.NewListener(netw(net.ParseIP(host)), addr)
}

// check if we are listening on the ipv6 or ipv4 address
func netw(addr net.IP) string {
	if addr.To4() == nil {
		return IPV6
	}
	return IPV4
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
