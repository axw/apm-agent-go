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
	require.Len(t, tx.Spans, 6)
	assert.Equal(t, "INSERT INTO products", tx.Spans[0].Name)
	assert.Equal(t, "SELECT FROM products", tx.Spans[1].Name)
	assert.Equal(t, "SELECT FROM products", tx.Spans[2].Name)
	assert.Equal(t, "UPDATE products", tx.Spans[3].Name)
	assert.Equal(t, "UPDATE products", tx.Spans[4].Name)
	assert.Equal(t, "DELETE FROM products", tx.Spans[5].Name)
}
