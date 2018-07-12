package apmbuffalo

import (
	"context"
	"net/http"

	"github.com/gobuffalo/buffalo"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/module/apmhttp"
	"github.com/elastic/apm-agent-go/shenanigans"
)

// Middleware returns a new Buffalo middleware handler for tracing
// requests and reporting errors.
//
// This middleware will report panics, but will propagate them back
// up the stack to be handled by standard buffalo PanicHandler.
//
// By default, the middleware will use elasticapm.DefaultTracer.
// Use WithTracer to specify an alternative tracer.
func Middleware(o ...Option) buffalo.MiddlewareFunc {
	opts := options{tracer: elasticapm.DefaultTracer}
	for _, o := range o {
		o(&opts)
	}
	return func(h buffalo.Handler) buffalo.Handler {
		m := &middleware{tracer: opts.tracer, handler: h}
		return m.handle
	}
}

type middleware struct {
	handler buffalo.Handler
	tracer  *elasticapm.Tracer
}

func (m *middleware) handle(c buffalo.Context) (handlerErr error) {
	if !m.tracer.Active() {
		return m.handler(c)
	}
	routeInfo, ok := c.Data()["current_route"].(buffalo.RouteInfo)
	if !ok {
		return m.handler(c)
	}

	req := c.Request()
	name := req.Method + " " + routeInfo.Path
	tx := m.tracer.StartTransaction(name, "request")
	defer tx.End()

	ctx := elasticapm.ContextWithTransaction(c, tx)
	req = apmhttp.RequestWithContext(ctx, req)

	body := m.tracer.CaptureHTTPRequestBody(req)
	w, resp := apmhttp.WrapResponseWriter(c.Response())
	overrideContext := overrideContext{
		Context: c,
		ctx:     ctx,
		req:     req,
		rw:      w,
	}
	defer func() {
		if v := recover(); v != nil {
			e := m.tracer.Recovered(v, tx)
			e.Context.SetHTTPRequest(req)
			e.Context.SetHTTPRequestBody(body)
			e.Send()
			apmhttp.SetTransactionContext(tx, req, resp, body, false)
			panic(v) // defer to buffalo's PanicHandler
		}
		if handlerErr != nil {
			e := m.tracer.NewError(handlerErr)
			e.Context.SetHTTPRequest(req)
			e.Context.SetHTTPRequestBody(body)
			e.Transaction = tx
			e.Send()
		}
		apmhttp.SetTransactionContext(tx, req, resp, body, true)
	}()

	shenanigans.Do(ctx, func() {
		handlerErr = m.handler(overrideContext)
	})
	return handlerErr
}

type options struct {
	tracer *elasticapm.Tracer
}

// Option sets options for tracing.
type Option func(*options)

// WithTracer returns an Option which sets t as the tracer
// to use for tracing server requests.
func WithTracer(t *elasticapm.Tracer) Option {
	if t == nil {
		panic("t == nil")
	}
	return func(o *options) {
		o.tracer = t
	}
}

type overrideContext struct {
	buffalo.Context
	ctx context.Context
	req *http.Request
	rw  http.ResponseWriter
}

func (oc overrideContext) Request() *http.Request {
	return oc.req
}

func (oc overrideContext) Response() http.ResponseWriter {
	return oc.rw
}

func (oc overrideContext) Value(key interface{}) interface{} {
	return oc.ctx.Value(key)
}
