package memberships

import (
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

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
