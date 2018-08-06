package apmgorm

import (
	"context"

	"github.com/jinzhu/gorm"
)

const apmContextKey = "elasticapm:context"

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
