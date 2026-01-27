package notifications

import (
	"context"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/google/uuid"
)

// Service defines notification list/read operations.
type Service interface {
	List(ctx context.Context, params ListParams) (*ListResult, error)
	MarkRead(ctx context.Context, storeID, notificationID uuid.UUID) error
	MarkAllRead(ctx context.Context, storeID uuid.UUID) (int64, error)
}

type service struct {
	repo Repository
}

// ListParams configures pagination for notifications.
type ListParams struct {
	StoreID    uuid.UUID
	Limit      int
	Cursor     string
	UnreadOnly bool
}

// ListResult wraps returned notifications and the cursor for the next page.
type ListResult struct {
	Items  []models.Notification `json:"items"`
	Cursor string                `json:"cursor"`
}

// NewService wires notifications dependencies.
func NewService(repo Repository) (Service, error) {
	if repo == nil {
		return nil, pkgerrors.New(pkgerrors.CodeDependency, "notifications repository required")
	}
	return &service{repo: repo}, nil
}

func (s *service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.StoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "active store id required")
	}

	query := listNotificationsParams{
		StoreID:    params.StoreID,
		Limit:      pagination.LimitWithBuffer(params.Limit),
		UnreadOnly: params.UnreadOnly,
	}
	if params.Cursor != "" {
		cursor, err := pagination.ParseCursor(params.Cursor)
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid cursor")
		}
		query.Cursor = cursor
	}

	rows, next, err := s.repo.List(ctx, query)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "list notifications")
	}

	cursor := ""
	if next != nil {
		cursor = pagination.EncodeCursor(*next)
	}

	return &ListResult{
		Items:  rows,
		Cursor: cursor,
	}, nil
}

func (s *service) MarkRead(ctx context.Context, storeID, notificationID uuid.UUID) error {
	if storeID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "store id required")
	}
	if notificationID == uuid.Nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "notification id required")
	}

	result, err := s.repo.MarkRead(ctx, storeID, notificationID, time.Now().UTC())
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "mark notification read")
	}
	if !result.Found {
		return pkgerrors.New(pkgerrors.CodeNotFound, "notification not found")
	}
	return nil
}

func (s *service) MarkAllRead(ctx context.Context, storeID uuid.UUID) (int64, error) {
	if storeID == uuid.Nil {
		return 0, pkgerrors.New(pkgerrors.CodeValidation, "store id required")
	}

	count, err := s.repo.MarkAllRead(ctx, storeID, time.Now().UTC())
	if err != nil {
		return 0, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "mark notifications read")
	}
	return count, nil
}
