package wishlist

import (
	"time"

	products "github.com/angelmondragon/packfinderz-backend/internal/products"
	"github.com/google/uuid"
)

// WishlistItemDTO wraps the product summary included in a wishlist row.
type WishlistItemDTO struct {
	Product   products.ProductSummary `json:"product"`
	CreatedAt time.Time               `json:"created_at"`
}

// WishlistItemsPageDTO returns a cursor-paginated wishlist view.
type WishlistItemsPageDTO struct {
	Items      []WishlistItemDTO          `json:"items"`
	Pagination products.ProductPagination `json:"pagination"`
}

// WishlistIDsDTO is a lightweight projection containing only product IDs plus pagination metadata.
type WishlistIDsDTO struct {
	ProductIDs []uuid.UUID                `json:"product_ids"`
	Pagination products.ProductPagination `json:"pagination"`
}
