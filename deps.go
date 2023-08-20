package http

import (
	"log/slog"

	"go.uber.org/zap"
)

type Configurer interface {
	Has(name string) bool
	UnmarshalKey(name string, out interface{}) error
}

type Logger interface {
	NamedLogger(name string) *slog.Logger
	NamedZapLogger(name string) *zap.Logger
}
