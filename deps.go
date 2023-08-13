package http

import "log/slog"

type Configurer interface {
	Has(name string) bool
	UnmarshalKey(name string, out interface{}) error
}

type Logger interface {
	NamedLogger(name string) *slog.Logger
}
