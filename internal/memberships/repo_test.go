//go:build db
// +build db

package memberships

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("PACKFINDERZ_DB_DSN")
	if dsn == "" {
		t.Skip("PACKFINDERZ_DB_DSN is not set")
	}

	conn, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return conn
}

func TestRepositoryMembershipFlow(t *testing.T) {
	conn := openTestDB(t)
	tx := conn.Begin()
	if tx.Error != nil {
		t.Fatalf("begin tx: %v", tx.Error)
	}
	t.Cleanup(func() {
		_ = tx.Rollback()
	})

	repo := NewRepository(tx)
	ctx := context.Background()

	user := &models.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("pf_test_%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		FirstName:    "Test",
		LastName:     "Member",
		IsActive:     true,
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	store := &models.Store{
		ID:                   uuid.New(),
		Type:                 enums.StoreTypeVendor,
		CompanyName:          "Repo Store",
		OwnerID:              user.ID,
		KYCStatus:            enums.KYCStatusVerified,
		SubscriptionActive:   true,
		DeliveryRadiusMeters: 100,
		Address: types.Address{
			Line1:      "123 Test Ave",
			City:       "Tulsa",
			State:      "OK",
			PostalCode: "74104",
			Country:    "US",
			Lat:        36.153984,
			Lng:        -95.992775,
		},
	}
	if err := tx.Create(store).Error; err != nil {
		t.Fatalf("create store: %v", err)
	}

	membership, err := repo.CreateMembership(ctx, store.ID, user.ID, enums.MemberRoleOwner, nil, enums.MembershipStatusActive)
	if err != nil {
		t.Fatalf("create membership: %v", err)
	}

	list, err := repo.ListUserStores(ctx, user.ID)
	if err != nil {
		t.Fatalf("list user stores: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 store, got %d", len(list))
	}
	if list[0].StoreName != store.CompanyName {
		t.Fatalf("expected store name %s, got %s", store.CompanyName, list[0].StoreName)
	}
	if list[0].Role != enums.MemberRoleOwner {
		t.Fatalf("unexpected role %s", list[0].Role)
	}

	exists, err := repo.UserHasRole(ctx, user.ID, store.ID, enums.MemberRoleOwner)
	if err != nil {
		t.Fatalf("check role: %v", err)
	}
	if !exists {
		t.Fatalf("expected user to have role owner")
	}

	other, err := repo.UserHasRole(ctx, user.ID, store.ID, enums.MemberRoleAdmin)
	if err != nil {
		t.Fatalf("check other role: %v", err)
	}
	if other {
		t.Fatal("expected user to not have admin role")
	}

	fetched, err := repo.GetMembership(ctx, user.ID, store.ID)
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if fetched.ID != membership.ID {
		t.Fatalf("expected membership id %s, got %s", membership.ID, fetched.ID)
	}

	if _, err := repo.CreateMembership(ctx, store.ID, user.ID, enums.MemberRoleAdmin, nil, enums.MembershipStatusActive); err == nil {
		t.Fatal("expected duplicate membership to fail")
	}
}
