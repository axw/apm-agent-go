// +build go1.9

package apmsql

import (
	"context"
	"database/sql/driver"
)

type stmtGo19 struct {
	namedValueChecker driver.NamedValueChecker
}

func (s *stmtGo19) init(in driver.Stmt) {
	s.namedValueChecker, _ = in.(driver.NamedValueChecker)
}

func (s *stmt) CheckNamedValue(nv *driver.NamedValue) error {
	if _, ok := nv.Value.(ContextValue); ok {
		// We extend the driver such that it supports
		// passing context via a ContextValue arg.
		return nil
	}
	if s.namedValueChecker != nil {
		return s.namedValueChecker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// ContextValue can be used to pass context into non-context database
// methods via statement arguments. This must be the final value in
// the list of args; it is an error to include it elsewhere in the
// list. It is an error to pass ContextValue to a native context method.
type ContextValue struct {
	context.Context
}
