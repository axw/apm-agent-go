package apmsql

import (
	"context"
	"database/sql/driver"
)

// ContextValue can be used to pass context into non-context database
// methods via statement arguments. ContextValue args will be ignored
// if passed to a native context method with a non-empty (background)
// context.
//
// If remove is true, the ContextValue will be removed from the args
// passed on to the underlying driver; otherwise it will be replaced
// with an empty string. Whether the value should be removed or
// replaced depends on the SQL dialect: those using bindvar syntax
// with ordinals should replace, while those using a syntax like '?'
// should remove.
//
// In general, this should be avoided in favour of passing context to
// the native context database/sql methods: ExecContext, QueryContext,
// etc. The ContextValue method does not (can not) cover methods like
// Ping or Prepare, which do not accept args.
func ContextValue(ctx context.Context, remove bool) interface{} {
	return contextValue{Context: ctx, remove: remove}
}

type contextValue struct {
	context.Context
	remove bool
}

// namedValueChecker is identical to driver.NamedValueChecker, existing
// for compatibility with Go 1.8.
type namedValueChecker interface {
	CheckNamedValue(*driver.NamedValue) error
}

func checkNamedValue(nv *driver.NamedValue, next namedValueChecker) (context.Context, error) {
	if ctxVal, ok := nv.Value.(contextValue); ok {
		// We extend the driver such that it supports
		// passing context via a ContextValue arg.
		return ctxVal.Context, driver.ErrRemoveArgument
	}
	if next != nil {
		return next.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// extractContext returns ctx (if non-background) or otherwise the final
// ContextValue arg if any are supplied. All ContextValue args will be
// either removed or replaced with "" to avoid upsetting the underlying driver.
func extractContext(ctx context.Context, args *[]driver.NamedValue) context.Context {
	isBackground := ctx == context.Background()
	for i := 0; i < len(*args); i++ {
		arg := (*args)[i]
		if ctxVal, ok := arg.Value.(contextValue); ok {
			if isBackground {
				ctx = ctxVal.Context
			}
			if !ctxVal.remove && i+1 < len(*args) {
				(*args)[i].Value = ""
				continue
			}
			ordinal := arg.Ordinal
			if i+1 < len(*args) {
				copy((*args)[i:], (*args)[i+1:])
			}
			*args = (*args)[:len(*args)-1]
			for i, arg := range *args {
				if arg.Ordinal > ordinal {
					arg.Ordinal--
					(*args)[i] = arg
				}
			}
			i--
		}
	}
	return ctx
}
