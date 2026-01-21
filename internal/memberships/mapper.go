package memberships

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

type storeUserRow struct {
	models.StoreMembership
	Email       string     `gorm:"column:email"`
	FirstName   string     `gorm:"column:first_name"`
	LastName    string     `gorm:"column:last_name"`
	LastLoginAt *time.Time `gorm:"column:last_login_at"`
}

type membershipWithStoreRow struct {
	models.StoreMembership
	StoreName string          `gorm:"column:store_name"`
	StoreType enums.StoreType `gorm:"column:store_type"`
}

func membershipWithStoreFromRow(row membershipWithStoreRow) MembershipWithStore {
	return MembershipWithStore{
		MembershipID:    row.ID,
		StoreID:         row.StoreID,
		UserID:          row.UserID,
		StoreName:       row.StoreName,
		StoreType:       row.StoreType,
		Role:            row.Role,
		Status:          row.Status,
		InvitedByUserID: copyUUIDPointer(row.InvitedByUserID),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func membershipRowsToDTO(rows []membershipWithStoreRow) []MembershipWithStore {
	out := make([]MembershipWithStore, 0, len(rows))
	for _, row := range rows {
		out = append(out, membershipWithStoreFromRow(row))
	}
	return out
}

func storeUsersFromRows(rows []storeUserRow) []StoreUserDTO {
	out := make([]StoreUserDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, storeUserFromRow(row))
	}
	return out
}

func storeUserFromRow(row storeUserRow) StoreUserDTO {
	return StoreUserDTO{
		MembershipID: row.ID,
		StoreID:      row.StoreID,
		UserID:       row.UserID,
		Email:        row.Email,
		FirstName:    row.FirstName,
		LastName:     row.LastName,
		Role:         row.Role,
		Status:       row.Status,
		CreatedAt:    row.CreatedAt,
		LastLoginAt:  row.LastLoginAt,
	}
}
