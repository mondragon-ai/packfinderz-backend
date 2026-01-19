package validators

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/go-playground/validator/v10"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		tag := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if tag == "" {
			return f.Name
		}
		return tag
	})
	return v
}

func DecodeJSONBody(r *http.Request, dest any) error {
	defer func() {
		io.Copy(io.Discard, r.Body)
	}()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid request body").WithDetails(map[string]any{"error": err.Error()})
	}
	if err := validate.Struct(dest); err != nil {
		return formatValidationErrors(err)
	}
	return nil
}

func formatValidationErrors(err error) *pkgerrors.Error {
	if errs, ok := err.(validator.ValidationErrors); ok {
		details := map[string]string{}
		for _, fieldErr := range errs {
			details[fieldErr.Field()] = validationMessage(fieldErr)
		}
		return pkgerrors.New(pkgerrors.CodeValidation, "validation failed").WithDetails(details)
	}
	return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "validation failed")
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "email":
		return "must be a valid email"
	}
	return "is invalid"
}
