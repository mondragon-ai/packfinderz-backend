package responses

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

func WriteSuccess(w http.ResponseWriter, data any) {
	WriteSuccessStatus(w, http.StatusOK, data)
}

func WriteSuccessStatus(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, types.SuccessEnvelope{Data: data})
}

func WriteError(ctx context.Context, logg *logger.Logger, w http.ResponseWriter, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}

	typed := pkgerrors.As(err)
	if typed == nil {
		typed = pkgerrors.Wrap(pkgerrors.CodeInternal, err, "unexpected error")
	}

	meta := pkgerrors.MetadataFor(typed.Code())

	msg := meta.PublicMessage
	switch typed.Code() {
	case pkgerrors.CodeValidation,
		pkgerrors.CodeForbidden,
		pkgerrors.CodeUnauthorized,
		pkgerrors.CodeNotFound,
		pkgerrors.CodeConflict,
		pkgerrors.CodeStateConflict,
		pkgerrors.CodeIdempotency,
		pkgerrors.CodeRateLimit:
		if m := typed.Message(); m != "" {
			msg = m
		}
	}

	payload := types.ErrorEnvelope{
		Error: types.APIError{
			Code:    string(typed.Code()),
			Message: msg,
		},
	}

	if meta.DetailsAllowed {
		if details := typed.Details(); details != nil {
			payload.Error.Details = details
		}
	}

	if logg != nil {
		dump := pkgerrors.Dump(err)

		fields := map[string]any{
			"error":         dump.TopMessage,
			"error_code":    dump.Code,
			"error_chain":   dump.Chain,
			"pg_code":       dump.PGCode,
			"pg_detail":     dump.PGDetail,
			"pg_message":    dump.PGMessage,
			"pg_table":      dump.PGTable,
			"pg_column":     dump.PGColumn,
			"pg_constraint": dump.PGConstraint,
		}

		if d := typed.Details(); d != nil {
			if dm, ok := d.(map[string]any); ok {
				if step, ok := dm["step"]; ok {
					fields["step"] = step
				}
			}
		}

		ctx = logg.WithFields(ctx, fields)
		logg.Error(ctx, "request.error", err)
	}

	writeJSON(w, meta.HTTPStatus, payload)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf(`{"level":"error","msg":"failed to encode response","err":"%v"}`, err)
	}
}
