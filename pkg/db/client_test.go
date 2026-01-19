package db

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testModel struct {
	ID   int
	Name string
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := conn.AutoMigrate(&testModel{}); err != nil {
		t.Fatalf("failed to migrate sqlite: %v", err)
	}
	return conn
}

func TestWithTx_CommitsAndRollbacks(t *testing.T) {
	db := newTestDB(t)
	client := &Client{conn: db}

	ctx := context.Background()
	if err := client.WithTx(ctx, func(tx *gorm.DB) error {
		return tx.Create(&testModel{Name: "committed"}).Error
	}); err != nil {
		t.Fatalf("WithTx commit failed: %v", err)
	}

	var count int64
	if err := db.Model(&testModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 record, got %d", count)
	}

	err := client.WithTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(&testModel{Name: "rolled"}).Error; err != nil {
			return err
		}
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected WithTx to return an error")
	}
	if err := db.Model(&testModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count failed after rollback: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected rollback to leave 1 record, got %d", count)
	}
}

func TestPing(t *testing.T) {
	db := newTestDB(t)
	client := &Client{conn: db}
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("unexpected ping error: %v", err)
	}
}
