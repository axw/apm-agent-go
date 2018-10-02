package apmbeego

import (
	"net/http"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/module/apmhttp"
)

func BeforeRouter(ctx *context.Context) {
	tracer := elasticapm.DefaultTracer
	tx, req := apmhttp.StartTransaction(tracer, "request", ctx.Request)
	tx.Context.SetFramework("beego", beego.VERSION)

	body := tracer.CaptureHTTPRequestBody(req)
	ctx.Input.SetData("apmbeego:body", body)
	ctx.Request = req
}

func AfterRouter(ctx *context.Context) {
	tx := elasticapm.TransactionFromContext(ctx.Request.Context())
	if tx == nil {
		return
	}
	if route, ok := ctx.Input.GetData("RouterPattern").(string); ok {
		tx.Name = route
	}
	body, _ := ctx.Input.GetData("apmbeego:body").(*elasticapm.BodyCapturer)
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
