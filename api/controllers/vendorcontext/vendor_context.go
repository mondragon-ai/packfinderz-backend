package vendorcontext

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

// ResolveVendorStoreID extracts the active store and enforces vendor access.
func ResolveVendorStoreID(r *http.Request) (uuid.UUID, error) {
	ctx := r.Context()
	storeID := middleware.StoreIDFromContext(ctx)
	if storeID == "" {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeForbidden, "store context required")
	}

	storeType, ok := middleware.StoreTypeFromContext(ctx)
	if !ok || storeType != enums.StoreTypeVendor {
		return uuid.Nil, pkgerrors.New(pkgerrors.CodeForbidden, "vendor access required")
	}

	id, err := uuid.Parse(storeID)
	if err != nil {
		return uuid.Nil, pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid store id")
	}
	return id, nil
}
