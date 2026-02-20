package errors

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

type ErrorDump struct {
	TopMessage string `json:"top_message"`
	Code       Code   `json:"code,omitempty"`

	Chain []string `json:"chain,omitempty"`

	PGCode       string `json:"pg_code,omitempty"`
	PGConstraint string `json:"pg_constraint,omitempty"`
	PGTable      string `json:"pg_table,omitempty"`
	PGColumn     string `json:"pg_column,omitempty"`
	PGDetail     string `json:"pg_detail,omitempty"`
	PGMessage    string `json:"pg_message,omitempty"`
}

func Dump(err error) ErrorDump {
	if err == nil {
		return ErrorDump{}
	}

	d := ErrorDump{
		TopMessage: err.Error(),
	}

	if te := As(err); te != nil {
		d.Code = te.Code()
	}

	for e := err; e != nil; e = errors.Unwrap(e) {
		d.Chain = append(d.Chain, fmt.Sprintf("%T: %v", e, e))
	}

	var pgxErr *pgconn.PgError
	if errors.As(err, &pgxErr) {
		d.PGCode = pgxErr.Code
		d.PGConstraint = pgxErr.ConstraintName
		d.PGTable = pgxErr.TableName
		d.PGColumn = pgxErr.ColumnName
		d.PGDetail = pgxErr.Detail
		d.PGMessage = pgxErr.Message
		return d
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		d.PGCode = string(pqErr.Code)
		d.PGConstraint = pqErr.Constraint
		d.PGTable = pqErr.Table
		d.PGColumn = pqErr.Column
		d.PGDetail = pqErr.Detail
		d.PGMessage = pqErr.Message
		return d
	}

	return d
}
