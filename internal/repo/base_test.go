package repo

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	return conn
}

func TestNewBaseStoresConnection(t *testing.T) {
	db := newTestDB(t)
	base := NewBase(db)

	if base.db != db {
		t.Fatalf("expected base db to match provided connection")
	}
}

func TestBaseDB_BindsContext(t *testing.T) {
	db := newTestDB(t)
	base := NewBase(db)

	ctx := context.WithValue(context.Background(), struct{}{}, "value")
	withCtx := base.DB(ctx)

	if withCtx == nil {
		t.Fatalf("expected non-nil DB when context provided")
	}
	if withCtx.Statement == nil {
		t.Fatalf("expected statement created after WithContext")
	}
	if withCtx.Statement.Context != ctx {
		t.Fatalf("expected context to flow through, got %v", withCtx.Statement.Context)
	}

	withoutCtx := base.DB(nil)
	if withoutCtx != db {
		t.Fatalf("expected nil context to return raw connection")
	}
}
