package errors

import (
	stdErrors "errors"
	"fmt"
	"net/http"
)

type Code string

const (
	CodeValidation    Code = "VALIDATION_ERROR"
	CodeUnauthorized  Code = "UNAUTHORIZED"
	CodeForbidden     Code = "FORBIDDEN"
	CodeNotFound      Code = "NOT_FOUND"
	CodeConflict      Code = "CONFLICT"
	CodeStateConflict Code = "STATE_CONFLICT"
	CodeIdempotency   Code = "IDEMPOTENCY_KEY_REUSED"
	CodeRateLimit     Code = "RATE_LIMIT_EXCEEDED"
	CodeInternal      Code = "INTERNAL_ERROR"
	CodeDependency    Code = "DEPENDENCY_ERROR"
)

type Metadata struct {
	HTTPStatus     int
	Retryable      bool
	PublicMessage  string
	DetailsAllowed bool
}

var metadataByCode = map[Code]Metadata{
	CodeValidation: {
		HTTPStatus:     http.StatusBadRequest,
		Retryable:      false,
		PublicMessage:  "validation failed",
		DetailsAllowed: true,
	},
	CodeUnauthorized: {
		HTTPStatus:     http.StatusUnauthorized,
		Retryable:      false,
		PublicMessage:  "authentication required",
		DetailsAllowed: false,
	},
	CodeForbidden: {
		HTTPStatus:     http.StatusForbidden,
		Retryable:      false,
		PublicMessage:  "access denied",
		DetailsAllowed: false,
	},
	CodeNotFound: {
		HTTPStatus:     http.StatusNotFound,
		Retryable:      false,
		PublicMessage:  "resource not found",
		DetailsAllowed: false,
	},
	CodeConflict: {
		HTTPStatus:     http.StatusConflict,
		Retryable:      false,
		PublicMessage:  "conflict detected",
		DetailsAllowed: false,
	},
	CodeStateConflict: {
		HTTPStatus:     http.StatusUnprocessableEntity,
		Retryable:      false,
		PublicMessage:  "state transition disallowed",
		DetailsAllowed: true,
	},
	CodeIdempotency: {
		HTTPStatus:     http.StatusConflict,
		Retryable:      false,
		PublicMessage:  "idempotency key reused",
		DetailsAllowed: true,
	},
	CodeRateLimit: {
		HTTPStatus:     http.StatusTooManyRequests,
		Retryable:      false,
		PublicMessage:  "rate limit exceeded",
		DetailsAllowed: false,
	},
	CodeInternal: {
		HTTPStatus:     http.StatusInternalServerError,
		Retryable:      true,
		PublicMessage:  "internal server error",
		DetailsAllowed: false,
	},
	CodeDependency: {
		HTTPStatus:     http.StatusServiceUnavailable,
		Retryable:      true,
		PublicMessage:  "dependency unavailable",
		DetailsAllowed: true,
	},
}

func MetadataFor(code Code) Metadata {
	if meta, ok := metadataByCode[code]; ok {
		return meta
	}
	return metadataByCode[CodeInternal]
}

type Error struct {
	code    Code
	message string
	details any
	cause   error
}

func New(code Code, message string) *Error {
	return &Error{code: code, message: message}
}

func Wrap(code Code, err error, message string) *Error {
	if err == nil {
		return New(code, message)
	}
	return &Error{code: code, message: message, cause: err}
}

func (e *Error) Code() Code {
	if e == nil {
		return CodeInternal
	}
	return e.code
}

func (e *Error) Message() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *Error) Details() any {
	if e == nil {
		return nil
	}
	return e.details
}

func (e *Error) WithDetails(details any) *Error {
	if e == nil {
		return nil
	}
	e.details = details
	return e
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func As(err error) *Error {
	if err == nil {
		return nil
	}
	var typed *Error
	if stdErrors.As(err, &typed) {
		return typed
	}
	return nil
}
