package reviews

import (
	"fmt"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/google/uuid"
)

// CreateReviewRequest defines the payload for POST /reviews.
type CreateReviewRequest struct {
	VendorStoreID string  `json:"vendor_store_id"`
	ProductID     *string `json:"product_id,omitempty"`
	OrderID       *string `json:"order_id,omitempty"`
	Rating        int16   `json:"rating"`
	Title         *string `json:"title,omitempty"`
	Body          *string `json:"body,omitempty"`
}

// ReviewResponse mirrors the review payload returned to clients.
type ReviewResponse struct {
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
	CreatedAt          string           `json:"created_at"`
	UpdatedAt          string           `json:"updated_at"`
}

// ReviewListResponse wraps a page of ReviewResponse entries plus pagination metadata.
type ReviewListResponse struct {
	Reviews    []ReviewResponse `json:"reviews"`
	Pagination ReviewPagination `json:"pagination"`
}

func (r CreateReviewRequest) ToInput(buyerStoreID, buyerUserID uuid.UUID) (CreateReviewInput, error) {
	if strings.TrimSpace(r.VendorStoreID) == "" {
		return CreateReviewInput{}, fmt.Errorf("vendor_store_id is required")
	}
	vendorID, err := uuid.Parse(r.VendorStoreID)
	if err != nil {
		return CreateReviewInput{}, fmt.Errorf("invalid vendor_store_id: %w", err)
	}

	input := CreateReviewInput{
		ReviewType:    enums.ReviewTypeStore,
		BuyerStoreID:  buyerStoreID,
		BuyerUserID:   buyerUserID,
		VendorStoreID: &vendorID,
		Rating:        r.Rating,
		Title:         r.Title,
		Body:          r.Body,
	}

	if r.ProductID != nil && *r.ProductID != "" {
		productID, err := uuid.Parse(*r.ProductID)
		if err != nil {
			return CreateReviewInput{}, fmt.Errorf("invalid product_id: %w", err)
		}
		input.ProductID = &productID
	}

	if r.OrderID != nil && *r.OrderID != "" {
		orderID, err := uuid.Parse(*r.OrderID)
		if err != nil {
			return CreateReviewInput{}, fmt.Errorf("invalid order_id: %w", err)
		}
		input.OrderID = &orderID
	}

	return input, nil
}

func ReviewToResponse(review *Review) ReviewResponse {
	resp := ReviewResponse{
		ID:                 review.ID,
		ReviewType:         review.ReviewType,
		BuyerStoreID:       review.BuyerStoreID,
		BuyerUserID:        review.BuyerUserID,
		VendorStoreID:      review.VendorStoreID,
		ProductID:          review.ProductID,
		OrderID:            review.OrderID,
		Rating:             review.Rating,
		Title:              review.Title,
		Body:               review.Body,
		IsVerifiedPurchase: review.IsVerifiedPurchase,
		IsVisible:          review.IsVisible,
		CreatedAt:          review.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          review.UpdatedAt.Format(time.RFC3339),
	}
	return resp
}

func ReviewListResultToResponse(result ReviewListResult) ReviewListResponse {
	responses := make([]ReviewResponse, 0, len(result.Reviews))
	for _, review := range result.Reviews {
		r := review
		responses = append(responses, ReviewToResponse(&r))
	}

	return ReviewListResponse{
		Reviews:    responses,
		Pagination: result.Pagination,
	}
}
