package reviews

import (
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

// Review represents a buyer-submitted rating/feedback entry.
type Review struct {
	ID                 uuid.UUID        `json:"id"`
	ReviewType         enums.ReviewType `json:"review_type"`
	BuyerStoreID       uuid.UUID        `json:"buyer_store_id"`
	BuyerUserID        uuid.UUID        `json:"buyer_user_id"`
	VendorStoreID      *uuid.UUID       `json:"vendor_store_id,omitempty"`
	ProductID          *uuid.UUID       `json:"product_id,omitempty"`
	OrderID            *uuid.UUID       `json:"order_id,omitempty"`
	Rating             int16            `json:"rating"`
	Title              *string          `json:"title,omitempty"`
	Body               *string          `json:"body,omitempty"`
	IsVerifiedPurchase bool             `json:"is_verified_purchase"`
	IsVisible          bool             `json:"is_visible"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// ReviewPagination mirrors the link-style pagination metadata used across the API.
type ReviewPagination struct {
	Page    int    `json:"page"`
	Total   int    `json:"total"`
	Current string `json:"current,omitempty"`
	First   string `json:"first,omitempty"`
	Last    string `json:"last,omitempty"`
	Prev    string `json:"prev,omitempty"`
	Next    string `json:"next,omitempty"`
}

// ReviewListResult wraps a page of reviews plus pagination metadata.
type ReviewListResult struct {
	Reviews    []Review         `json:"reviews"`
	Pagination ReviewPagination `json:"pagination"`
}

// CreateReviewInput carries the fields required to persist a new review.
type CreateReviewInput struct {
	ReviewType         enums.ReviewType
	BuyerStoreID       uuid.UUID
	BuyerUserID        uuid.UUID
	VendorStoreID      *uuid.UUID
	ProductID          *uuid.UUID
	OrderID            *uuid.UUID
	Rating             int16
	Title              *string
	Body               *string
	IsVerifiedPurchase bool
	IsVisible          *bool
}
