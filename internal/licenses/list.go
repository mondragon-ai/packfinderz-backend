package licenses

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgpagination "github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
)

type ListParams struct {
	StoreID uuid.UUID
	pkgpagination.Params
}

type ListResult struct {
	Items  []ListItem `json:"items"`
	Cursor string     `json:"cursor"`
}

type ListItem struct {
	ID             uuid.UUID           `json:"id"`
	StoreID        uuid.UUID           `json:"store_id"`
	UserID         uuid.UUID           `json:"user_id"`
	Status         enums.LicenseStatus `json:"status"`
	MediaID        uuid.UUID           `json:"media_id"`
	IssuingState   string              `json:"issuing_state"`
	IssueDate      *time.Time          `json:"issue_date"`
	ExpirationDate *time.Time          `json:"expiration_date"`
	Type           enums.LicenseType   `json:"type"`
	Number         string              `json:"number"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
	SignedURL      string              `json:"signed_url,omitempty"`
}

type listQuery struct {
	storeID uuid.UUID
	limit   int
	cursor  *pkgpagination.Cursor
}

func toListItem(m models.License) ListItem {
	return ListItem{
		ID:             m.ID,
		StoreID:        m.StoreID,
		UserID:         m.UserID,
		Status:         m.Status,
		MediaID:        m.MediaID,
		IssuingState:   m.IssuingState,
		IssueDate:      m.IssueDate,
		ExpirationDate: m.ExpirationDate,
		Type:           m.Type,
		Number:         m.Number,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
