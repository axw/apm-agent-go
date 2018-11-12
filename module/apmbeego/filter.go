package apmbeego

import (
	"fmt"
	"net/http"
	"runtime/pprof"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
)

func BeforeRouter(ctx *context.Context) {
	tracer := apm.DefaultTracer
	tx, req := apmhttp.StartTransaction(tracer, "request", ctx.Request)
	tx.Context.SetFramework("beego", beego.VERSION)

	body := tracer.CaptureHTTPRequestBody(req)
	ctx.Input.SetData("apmbeego:body", body)
	ctx.Request = req

	traceContext := tx.TraceContext()
	pprof.SetGoroutineLabels(pprof.WithLabels(
		ctx.Request.Context(),
		pprof.Labels("trace_context", fmt.Sprintf(
			"%s-%s-%d", traceContext.Trace, traceContext.Span, traceContext.Options,
		)),
	))
}

func AfterRouter(ctx *context.Context) {
	tx := apm.TransactionFromContext(ctx.Request.Context())
	if tx == nil {
		return
	}
	pprof.SetGoroutineLabels(ctx.Request.Context()) // reset labels
	if route, ok := ctx.Input.GetData("RouterPattern").(string); ok {
		tx.Name = route
	}
	body, _ := ctx.Input.GetData("apmbeego:body").(*apm.BodyCapturer)
	statusCode := ctx.ResponseWriter.Status
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	apmhttp.SetTransactionContext(tx, ctx.Request, &apmhttp.Response{
		StatusCode: statusCode,
		Headers:    ctx.ResponseWriter.Header(),
	}, body)
	tx.End()
}

func InsertFilters() {
	beego.InsertFilter("*", beego.BeforeRouter, BeforeRouter, false)
	beego.InsertFilter("*", beego.FinishRouter, AfterRouter, false)
}
