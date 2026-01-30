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

func safeValidateStruct(dest any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[VALIDATION] Running dive-tag scan to find likely offender...\n")
			scanDiveTags(dest)
			err = fmt.Errorf("validation panic: %v", r)
		}
	}()

	scanDiveTags(dest)

	if e := validate.Struct(dest); e != nil {
		return e
	}
	return nil
}

func scanDiveTags(dest any) {
	t := reflect.TypeOf(dest)
	v := reflect.ValueOf(dest)

	for t != nil && t.Kind() == reflect.Ptr {
		if v.IsValid() && !v.IsNil() {
			v = v.Elem()
		}
		t = t.Elem()
	}

	if t == nil || t.Kind() != reflect.Struct {
		fmt.Printf("[VALIDATION] scanDiveTags: dest is not a struct (type=%T kind=%v)\n", dest, t.Kind())
		return
	}

	walkStructFields(t, "", func(path string, ft reflect.StructField) {
		tag := ft.Tag.Get("validate")
		if tag == "" {
			return
		}
		if !strings.Contains(tag, "dive") {
			return
		}

		kind := ft.Type.Kind()
		elemKind := kind

		if kind == reflect.Ptr {
			elemKind = ft.Type.Elem().Kind()
		}

		isDiveable := elemKind == reflect.Slice || elemKind == reflect.Array || elemKind == reflect.Map

		if !isDiveable {
			fmt.Printf("[VALIDATION] ❌ dive tag on NON-diveable field: %s type=%s kind=%v validate=%q\n",
				path, ft.Type.String(), ft.Type.Kind(), tag)
		} else {
			fmt.Printf("[VALIDATION] ✅ dive tag OK: %s type=%s validate=%q\n", path, ft.Type.String(), tag)
		}
	})
}

func walkStructFields(t reflect.Type, prefix string, fn func(path string, ft reflect.StructField)) {
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)

		if ft.PkgPath != "" {
			continue
		}
		name := ft.Name
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		fn(path, ft)

		ftType := ft.Type
		for ftType.Kind() == reflect.Ptr {
			ftType = ftType.Elem()
		}
		if ftType.Kind() == reflect.Struct {
			if ftType != t {
				walkStructFields(ftType, path, fn)
			}
		}
	}
}

func DecodeJSONBody(r *http.Request, dest any) error {
	defer func() {
		_, _ = io.Copy(io.Discard, r.Body)
	}()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dest); err != nil {
		return pkgerrors.Wrap(pkgerrors.CodeValidation, err, "invalid request body").
			WithDetails(map[string]any{"error": err.Error()})
	}
	if err := safeValidateStruct(dest); err != nil {
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
