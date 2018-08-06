package apmgorm

import (
	"database/sql"
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go/module/apmsql"
)

// Open returns a *gorm.DB for the given dialect and arguments,
// using apmsql.Open to provide an instrumented database connection.
// The returned *gorm.DB will have callbacks registered with
// RegisterCallbacks.
//
// Open accepts the following signatures:
//  - a datasource name (i.e. the second argument to sql.Open)
//  - a driver name and a datasource name
//  - a *sql.DB, or some other type with the same interface
//
// If a *sql.DB is passed in, it must be using a driver instrumented
// with apmsql, i.e. by having been obtained via apmsql.Open, or
// via sql.Open using the instrumented driver name. If this cannot be
// confirmed, then an error will be returned.
func Open(dialect string, args ...interface{}) (*gorm.DB, error) {
	switch len(args) {
	case 1:
		switch arg0 := args[0].(type) {
		case string:
			driver := apmsql.DriverPrefix + dialect
			args = []interface{}{driver, arg0}
		case gorm.SQLCommon:
			db, ok := arg0.(*sql.DB)
			if !ok || !apmsql.IsWrapped(db.Driver()) {
				return nil, errors.New("expected a *sql.DB with a driver wrapped by apmsql.Wrap")
			}
		}
	case 2:
		if driver, ok := args[0].(string); ok {
			args[0] = apmsql.DriverPrefix + driver
		}
	}
	db, err := gorm.Open(dialect, args...)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	RegisterCallbacks(db)
	return db, nil
}

type instrumentedDialect struct {
	gorm.Dialect
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
		processor().Before(name).Register(fmt.Sprintf("%s:before:%s", callbackPrefix, name), beforeCallback)
		processor().After(name).Register(fmt.Sprintf("%s:after:%s", callbackPrefix, name), afterCallback)
	}
}

func beforeCallback(scope *gorm.Scope) {
	ctx, ok := scopeContext(scope)
	if !ok {
		return
	}
	// TODO(axw) create a span for gorm, add the
	// resulting span context to the vars.
	placeholder := scope.Dialect().BindVar(len(scope.SQLVars) + 1)
	remove := placeholder == "$$$"
	scope.SQLVars = append(scope.SQLVars, apmsql.ContextValue(ctx, remove))
}

func afterCallback(scope *gorm.Scope) {
	if _, ok := scopeContext(scope); !ok {
		return
	}
	scope.SQLVars = scope.SQLVars[:len(scope.SQLVars)-1]
}
