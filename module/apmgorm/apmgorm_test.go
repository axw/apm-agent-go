package apmgorm_test

import (
	"context"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go/apmtest"
	"github.com/elastic/apm-agent-go/module/apmgorm"
	_ "github.com/elastic/apm-agent-go/module/apmgorm/dialects/sqlite"
)

type Product struct {
	gorm.Model
	Code  string
	Price uint
}

func TestWithContext(t *testing.T) {
	tx, _ := apmtest.WithTransaction(func(ctx context.Context) {
		db, err := apmgorm.Open("sqlite3", ":memory:")
		require.NoError(t, err)
		defer db.Close()

		// Migrate the schema
		db.AutoMigrate(&Product{})

		// Create
		db.Create(&Product{Code: "L1212", Price: 1000})

		// Read
		var product Product
		db.First(&product, 1)                   // find product with id 1
		db.First(&product, "code = ?", "L1212") // find product with code l1212

		// Update - update product's price to 2000
		db.Model(&product).Update("Price", 2000)

		// Delete - delete product
		db.Delete(&product)
	})
	_ = tx
}
