package apmgorm_test

import (
	"context"
	"os"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go/apmtest"
	"github.com/elastic/apm-agent-go/module/apmgorm"
	_ "github.com/elastic/apm-agent-go/module/apmgorm/dialects/mysql"
	_ "github.com/elastic/apm-agent-go/module/apmgorm/dialects/postgres"
	_ "github.com/elastic/apm-agent-go/module/apmgorm/dialects/sqlite"
)

type Product struct {
	gorm.Model
	Code  string
	Price uint
}

func TestWithContext(t *testing.T) {
	t.Run("sqlite3", func(t *testing.T) {
		testWithContext(t, "sqlite3", ":memory:")
	})

	if os.Getenv("PGHOST") == "" {
		t.Logf("PGHOST not specified, skipping")
	} else {
		t.Run("postgres", func(t *testing.T) {
			testWithContext(t, "postgres", "user=postgres password=hunter2 dbname=test_db sslmode=disable")
		})
	}

	if mysqlHost := os.Getenv("MYSQL_HOST"); mysqlHost == "" {
		t.Logf("MYSQL_HOST not specified, skipping")
	} else {
		t.Run("mysql", func(t *testing.T) {
			testWithContext(t, "mysql", "root:hunter2@tcp("+mysqlHost+")/test_db")
		})
	}
}

func testWithContext(t *testing.T, dialect string, args ...interface{}) {
	tx, _ := apmtest.WithTransaction(func(ctx context.Context) {
		db, err := apmgorm.Open(dialect, args...)
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
