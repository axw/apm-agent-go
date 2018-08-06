package apmgorm

import (
	"context"
	"fmt"

	"github.com/jinzhu/gorm"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/module/apmsql"
)

const (
	apmContextKey = "elasticapm:context"
)

// WithContext returns a copy of db with ctx recorded for use by
// the callbacks registered via RegisterCallbacks.
func WithContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	return db.Set(apmContextKey, ctx)
}

func scopeContext(scope *gorm.Scope) (context.Context, bool) {
	value, ok := scope.Get(apmContextKey)
	if !ok {
		return nil, false
	}
	ctx, ok := value.(context.Context)
	return ctx, ok
}

// RegisterCallbacks registers callbacks on db for reporting spans
// to Elastic APM. This is called automatically by apmgorm.Open;
// it is provided for cases where a *gorm.DB is acquired by other
// means.
func RegisterCallbacks(db *gorm.DB) {
	const callbackPrefix = "elasticapm"

	callbackProcessors := map[string]func() *gorm.CallbackProcessor{
		"gorm:create": func() *gorm.CallbackProcessor { return db.Callback().Create() },
		"gorm:delete": func() *gorm.CallbackProcessor { return db.Callback().Delete() },
		"gorm:query":  func() *gorm.CallbackProcessor { return db.Callback().Query() },
		"gorm:update": func() *gorm.CallbackProcessor { return db.Callback().Update() },
	}
	for name, processor := range callbackProcessors {
		processor().Before(name).Register(fmt.Sprintf("%s:before:%s", callbackPrefix, name), newBeforeCallback(name))
		processor().After(name).Register(fmt.Sprintf("%s:after:%s", callbackPrefix, name), afterCallback)
	}
}

func newBeforeCallback(gormOp string) func(*gorm.Scope) {
	return func(scope *gorm.Scope) {
		ctx, ok := scopeContext(scope)
		if !ok {
			return
		}
		span, ctx := elasticapm.StartSpan(ctx, gormOp+" "+scope.TableName(), gormOp)
		if span.Dropped() {
			return
		}
		scope = scope.Set(apmContextKey, ctx)
		placeholder := scope.Dialect().BindVar(len(scope.SQLVars) + 1)
		remove := placeholder == "$$$"
		scope.SQLVars = append(scope.SQLVars, apmsql.ContextValue(ctx, remove))
	}
}

func afterCallback(scope *gorm.Scope) {
	ctx, ok := scopeContext(scope)
	if !ok {
		return
	}
	fmt.Println("SQL:", scope.SQL, scope.SQLVars)
	span.End()
	scope.SQLVars = scope.SQLVars[:len(scope.SQLVars)-1]
}
