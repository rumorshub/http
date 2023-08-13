package http

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"sync"

	"github.com/roadrunner-server/endure/v2/dep"
	"github.com/roadrunner-server/errors"

	"github.com/rumorshub/http/config"
	"github.com/rumorshub/http/middleware"
	httpServer "github.com/rumorshub/http/servers/http"
	httpsServer "github.com/rumorshub/http/servers/https"
)

const (
	PluginName        = "http"
	MB         uint64 = 1024 * 1024
)

type internalServer interface {
	Start(map[string]middleware.Middleware, []string) error
	GetServer() *http.Server
	Stop()
}

type Plugin struct {
	mu sync.RWMutex

	log    *slog.Logger
	stdLog *log.Logger

	cfg *config.Config

	mdwr    map[string]middleware.Middleware
	handler http.Handler
	servers []internalServer
}

func (p *Plugin) Init(cfg Configurer, logger Logger) error {
	const op = errors.Op("http_plugin_init")
	if !cfg.Has(PluginName) {
		return errors.E(op, errors.Disabled)
	}

	if err := cfg.UnmarshalKey(PluginName, &p.cfg); err != nil {
		return errors.E(op, err)
	}

	if err := p.cfg.InitDefaults(); err != nil {
		return errors.E(op, err)
	}

	if !p.cfg.EnableHTTP() && !p.cfg.EnableTLS() {
		return errors.E(op, errors.Disabled)
	}

	p.log = logger.NamedLogger(PluginName)
	p.stdLog = log.New(NewStdAdapter(p.log), "http_plugin: ", log.Ldate|log.Ltime|log.LUTC)
	p.mdwr = make(map[string]middleware.Middleware)
	p.servers = make([]internalServer, 0, 2)
	p.handler = http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	return nil
}

func (p *Plugin) Serve() chan error {
	errCh := make(chan error, 2)
	var err error

	err = p.initServers()
	if err != nil {
		errCh <- err
		return errCh
	}

	p.applyBundledMiddleware()

	for i := 0; i < len(p.servers); i++ {
		go func(i int) {
			errSt := p.servers[i].Start(p.mdwr, p.cfg.Middleware)
			if errSt != nil {
				errCh <- errSt
				return
			}
		}(i)
	}

	return errCh
}

func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	doneCh := make(chan struct{}, 1)

	go func() {
		for i := 0; i < len(p.servers); i++ {
			if p.servers[i] != nil {
				p.servers[i].Stop()
			}
		}
		doneCh <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		return nil
	}
}

func (p *Plugin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	p.handler.ServeHTTP(w, r)
	p.mu.RUnlock()

	_ = r.Body.Close()
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) Collects() []*dep.In {
	return []*dep.In{
		dep.Fits(func(pp interface{}) {
			mdwr := pp.(middleware.Middleware)

			p.mu.Lock()
			p.mdwr[mdwr.Name()] = mdwr
			p.mu.Unlock()
		}, (*middleware.Middleware)(nil)),
		dep.Fits(func(pp interface{}) {
			mdwes := pp.(middleware.Middlewares)

			p.mu.Lock()
			for _, mdwr := range mdwes.HTTPMiddlewares() {
				p.mdwr[mdwr.(middleware.Middleware).Name()] = mdwr.(middleware.Middleware)
			}
			p.mu.Unlock()
		}, (*middleware.Middleware)(nil)),
		dep.Fits(func(pp interface{}) {
			handler := pp.(http.Handler)

			p.mu.Lock()
			p.handler = handler
			p.mu.Unlock()
		}, (*http.Handler)(nil)),
	}
}

func (p *Plugin) initServers() error {
	if p.cfg.EnableHTTP() {
		p.servers = append(p.servers, httpServer.NewHTTPServer(p, p.cfg, p.stdLog, p.log))
	}

	if p.cfg.EnableTLS() {
		https, err := httpsServer.NewHTTPSServer(p, p.cfg.SSL, p.cfg.HTTP2, p.stdLog, p.log)
		if err != nil {
			return err
		}

		p.servers = append(p.servers, https)
	}

	return nil
}

func (p *Plugin) applyBundledMiddleware() {
	for i := 0; i < len(p.servers); i++ {
		serv := p.servers[i].GetServer()
		serv.Handler = middleware.MaxRequestSize(serv.Handler, p.cfg.MaxRequestSize*MB)
		serv.Handler = middleware.NewLogMiddleware(serv.Handler, p.log)
	}
}
