package apperr

import (
	"errors"
	"fmt"
)

type Code string

const (
	Unauthorized  Code = "unauthorized"
	Forbidden     Code = "forbidden"
	NotFound      Code = "not_found"
	Conflict      Code = "conflict"
	Invalid       Code = "invalid"
	Unverified    Code = "unverified"
	Gone          Code = "gone"
	TooMany       Code = "too_many"
	Unavailable   Code = "unavailable"
	Internal      Code = "internal"
	NotAcceptable Code = "not_acceptable"
)

type Error struct {
	Code Code
	Msg  string
}

func (e *Error) Error() string {
	if e.Msg == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func New(code Code, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

func Is(err error, code Code) bool {
	got, ok := As(err)
	return ok && got.Code == code
}

func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
