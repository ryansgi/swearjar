// Package bind provides JSON bind and validation helpers for handlers
package bind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"

	perr "swearjar/internal/platform/errors"
	"swearjar/internal/platform/logger"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

// ctxKey is a tiny context key for stashing parsed payloads
type ctxKey uint8

const bindJSONPayloadKey ctxKey = iota

// FieldLevel aliases validator.FieldLevel
type FieldLevel = validator.FieldLevel

// UT aliases ut.Translator
type UT = ut.Translator

// FieldError aliases validator.FieldError
type FieldError = validator.FieldError

// ValidatorSvc holds a singleton validator and translator
type ValidatorSvc struct {
	Validator  *validator.Validate
	Translator ut.Translator
}

var (
	vOnce    sync.Once
	vSvc     *ValidatorSvc
	jsonMore = func(dec *json.Decoder) bool { return dec.More() } // seam
)

// Init initializes the singleton validator with english translations and json tag names
func Init() *ValidatorSvc {
	vOnce.Do(func() {
		enLoc := en.New()
		uni := ut.New(enLoc, enLoc)
		trans, _ := uni.GetTranslator("en")

		v := validator.New(validator.WithRequiredStructEnabled())

		// prefer json tag names in messages
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			tag := fld.Tag.Get("json")
			if tag == "-" || tag == "" {
				return fld.Name
			}
			if idx := strings.Index(tag, ","); idx >= 0 {
				tag = tag[:idx]
			}
			return tag
		})

		_ = en_translations.RegisterDefaultTranslations(v, trans)

		// short messages for min and max
		registerShortMin(v, trans)
		registerShortMax(v, trans)

		// common custom tag
		registerCommaInts(v, trans)

		vSvc = &ValidatorSvc{Validator: v, Translator: trans}
	})
	return vSvc
}

// Get returns the validator singleton, initializing on first use
func Get() *ValidatorSvc {
	if vSvc == nil {
		return Init()
	}
	return vSvc
}

// RegisterValidation registers a custom tag
func RegisterValidation(tag string, fn validator.Func) error {
	return Get().Validator.RegisterValidation(tag, fn)
}

// JSONOptions controls parsing behavior
type JSONOptions struct {
	MaxBytes        int64 // default 1MB
	DisallowUnknown bool  // default true
	AllowEmptyBody  bool  // default false
}

func defaultJSONOptions() JSONOptions {
	return JSONOptions{
		MaxBytes:        1 << 20,
		DisallowUnknown: true,
		AllowEmptyBody:  false,
	}
}

// ParseJSON decodes JSON into T, validates it, and maps failures to project errors
func ParseJSON[T any](r *http.Request, opts ...JSONOptions) (T, error) {
	var zero T
	o := defaultJSONOptions()
	if len(opts) > 0 {
		o = opts[0]
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Get().Error().Err(err).Msg("failed to close request body")
		}
	}()

	var reader io.Reader

	if !o.AllowEmptyBody {
		buf := make([]byte, 1)
		n, _ := r.Body.Read(buf)
		if n == 0 {
			// Tolerate empty body for safe/idempotent methods
			switch r.Method {
			case http.MethodGet, http.MethodDelete, http.MethodHead, http.MethodOptions:
				return zero, nil
			}
			return zero, perr.JSONErrf("empty body")
		}
		combined := io.MultiReader(bytes.NewReader(buf[:n]), r.Body)
		if o.MaxBytes > 0 {
			reader = io.LimitReader(combined, o.MaxBytes)
		} else {
			reader = combined
		}
	} else {
		if o.MaxBytes > 0 {
			reader = io.LimitReader(r.Body, o.MaxBytes)
		} else {
			reader = r.Body
		}
	}

	dec := json.NewDecoder(reader)
	if o.DisallowUnknown {
		dec.DisallowUnknownFields()
	}

	var dst T
	if err := dec.Decode(&dst); err != nil {
		// Treat EOF as acceptable when empty bodies are allowed
		if o.AllowEmptyBody && errors.Is(err, io.EOF) {
			return dst, nil
		}
		return zero, perr.JSONErrf("invalid JSON: %v", err)
	}

	if jsonMore(dec) {
		return zero, perr.JSONErrf("unexpected trailing data")
	}

	if err := Get().Validator.Struct(dst); err != nil {
		if inv, ok := err.(*validator.InvalidValidationError); ok {
			log := logger.Get()
			log.Error().Err(inv).Msg("validator internal error")
			return zero, perr.JSONErrf("validation error")
		}
		_, msg := ValidationFieldAndMessage(err)
		return zero, perr.Newf(perr.ErrorCodeValidation, "%s", msg) // field can be attached by caller if needed
	}

	return dst, nil
}

// JSON parses JSON into T and stores a pointer on the request context for downstream handler use
func JSON[T any](opts ...JSONOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			val, err := ParseJSON[T](r, opts...)
			if err != nil {
				// delegate error writing to caller. Keep this middleware transport-agnostic
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			ctx := context.WithValue(r.Context(), bindJSONPayloadKey, &val)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext retrieves the bound payload if present
func FromContext[T any](r *http.Request) *T {
	v, _ := r.Context().Value(bindJSONPayloadKey).(*T)
	return v
}

// ValidationFieldAndMessage returns the first field and translated message
func ValidationFieldAndMessage(err error) (field, message string) {
	if err == nil {
		return "", ""
	}
	if inv, ok := err.(*validator.InvalidValidationError); ok {
		return "", inv.Error()
	}
	if verrs, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range verrs {
			return fe.Field(), fe.Translate(Get().Translator)
		}
	}
	return "", err.Error()
}

// As re-exports errors.As to reduce import noise at call sites
func As(err error, target any) bool { return errors.As(err, target) }

// custom translations with short messages

func registerShortMin(v *validator.Validate, trans ut.Translator) {
	_ = v.RegisterTranslation("min", trans,
		func(ut ut.Translator) error {
			return ut.Add("min", "{0} must be at least {1}", true)
		},
		func(ut ut.Translator, fe validator.FieldError) string {
			msg, _ := ut.T("min", fe.Field(), fe.Param())
			return msg
		},
	)
}

func registerShortMax(v *validator.Validate, trans ut.Translator) {
	_ = v.RegisterTranslation("max", trans,
		func(ut ut.Translator) error {
			return ut.Add("max", "{0} must be at most {1}", true)
		},
		func(ut ut.Translator, fe validator.FieldError) string {
			msg, _ := ut.T("max", fe.Field(), fe.Param())
			return msg
		},
	)
}

func registerCommaInts(v *validator.Validate, trans ut.Translator) {
	_ = v.RegisterTranslation("comma_ints", trans,
		func(ut ut.Translator) error {
			return ut.Add("comma_ints", "{0} must be a comma-separated list of integers", true)
		},
		func(ut ut.Translator, fe validator.FieldError) string {
			msg, _ := ut.T("comma_ints", fe.Field())
			return msg
		},
	)
}
