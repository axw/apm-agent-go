package apmgorm

import (
	"database/sql"

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
