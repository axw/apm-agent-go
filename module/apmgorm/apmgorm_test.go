package apmgorm_test

import (
	"context"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
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
		db = apmgorm.WithContext(ctx, db)

		db.AutoMigrate(&Product{})
		db.Create(&Product{Code: "L1212", Price: 1000})

		var product Product
		db.First(&product, 1)                   // find product with id 1
		db.First(&product, "code = ?", "L1212") // find product with code l1212
		db.Model(&product).Update("Price", 2000)
		db.Delete(&product)            // soft
		db.Unscoped().Delete(&product) // hard
	})
	require.NotEmpty(t, tx.Spans)

	spanNames := make([]string, len(tx.Spans))
	for i, span := range tx.Spans {
		spanNames[i] = span.Name
	}
	assert.Equal(t, []string{
		"gorm:create products",
		"INSERT INTO products",

		"gorm:query products",
		"SELECT FROM products",

		"gorm:query products",
		"SELECT FROM products",

		"gorm:update products",
		"UPDATE products",

		// soft delete
		"gorm:delete products",
		"UPDATE products",

		"gorm:delete products",
		"DELETE FROM products",
	}, spanNames)
}
