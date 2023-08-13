package middleware

import "net/http"

type Middleware interface {
	Name() string
	Middleware(next http.Handler) http.Handler
}

type Middlewares interface {
	HTTPMiddlewares() []interface{}
}
