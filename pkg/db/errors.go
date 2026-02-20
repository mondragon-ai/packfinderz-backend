package db

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

func IsUniqueViolation(err error, constraintName string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if constraintName != "" {
		return strings.Contains(msg, constraintName)
	}
	return strings.Contains(msg, "duplicate key value")
}

func MapPGError(err error) error {
	if err == nil {
		return nil
	}

	var pgxErr *pgconn.PgError
	if errors.As(err, &pgxErr) {
		switch pgxErr.Code {
		case "23505":
			return pkgerrors.Wrap(pkgerrors.CodeConflict, err, "duplicate value violates unique constraint").
				WithDetails(map[string]any{"constraint": pgxErr.ConstraintName})
		case "23503":
			return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid reference").
				WithDetails(map[string]any{"constraint": pgxErr.ConstraintName})
		case "23502":
			return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "missing required field").
				WithDetails(map[string]any{"column": pgxErr.ColumnName})
		case "22P02":
			return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid input format").
				WithDetails(map[string]any{
					"pg_code": pgxErr.Code,
					"table":   pgxErr.TableName,
					"column":  pgxErr.ColumnName,
					"detail":  pgxErr.Detail,
					"message": pgxErr.Message,
				})
		default:
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "postgres error").
				WithDetails(map[string]any{"pg_code": pgxErr.Code, "constraint": pgxErr.ConstraintName})
		}
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		code := string(pqErr.Code)
		switch code {
		case "23505":
			return pkgerrors.Wrap(pkgerrors.CodeConflict, err, "duplicate value violates unique constraint").
				WithDetails(map[string]any{"constraint": pqErr.Constraint})
		case "23503":
			return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid reference").
				WithDetails(map[string]any{"constraint": pqErr.Constraint})
		case "23502":
			return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "missing required field").
				WithDetails(map[string]any{"column": pqErr.Column})
		case "22P02":
			return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid input format").
				WithDetails(map[string]any{
					"pg_code": code,
					"table":   pqErr.Table,
					"column":  pqErr.Column,
					"detail":  pqErr.Detail,
					"message": pqErr.Message,
				})
		default:
			return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "postgres error").
				WithDetails(map[string]any{"pg_code": code, "constraint": pqErr.Constraint})
		}
	}

	return pkgerrors.Wrap(pkgerrors.CodeDependency, err, "db error")
}
