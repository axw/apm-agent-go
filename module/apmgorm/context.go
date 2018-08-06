package apmgorm

import (
	"context"

	"github.com/jinzhu/gorm"
)

const apmContextKey = "elasticapm:context"

// TODO
func WithContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	db = db.Set(apmContextKey, ctx)
	return db
}

func scopeContext(scope *gorm.Scope) (context.Context, bool) {
	value, ok := scope.Get(apmContextKey)
	if !ok {
		return nil, false
	}
	ctx, ok := value.(context.Context)
	return ctx, ok
}
