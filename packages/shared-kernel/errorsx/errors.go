package errorsx

import (
	"errors"
	"fmt"
)

type Kind string

const (
	KindValidation Kind = "validation"
	KindNotFound   Kind = "not_found"
	KindForbidden  Kind = "forbidden"
)

type Error struct {
	Kind    Kind
	Message string
	Details map[string]string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func New(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, Cause: cause}
}

func Validation(message string, details map[string]string) *Error {
	return &Error{Kind: KindValidation, Message: message, Details: cloneDetails(details)}
}

func NotFound(message string, cause error) *Error {
	return New(KindNotFound, message, cause)
}

func Forbidden(message string, cause error) *Error {
	return New(KindForbidden, message, cause)
}

func IsKind(err error, kind Kind) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Kind == kind
}

func DetailsOf(err error) map[string]string {
	var e *Error
	if !errors.As(err, &e) {
		return nil
	}
	if e.Kind != KindValidation {
		return nil
	}
	return cloneDetails(e.Details)
}

func cloneDetails(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
