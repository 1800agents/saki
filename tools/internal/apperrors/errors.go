package apperrors

import (
	"errors"
	"fmt"
)

// Code identifies broad classes of internal tool failures.
type Code string

const (
	CodeInvalidInput    Code = "invalid_input"
	CodeConfig          Code = "config_error"
	CodeTemplate        Code = "template_error"
	CodeDocker          Code = "docker_error"
	CodeControlPlane    Code = "control_plane_error"
	CodeControlPlaneAPI Code = "control_plane_api_error"
	CodeTimeout         Code = "timeout"
	CodeInternal        Code = "internal_error"
)

// Coded is implemented by errors that expose a stable internal code.
type Coded interface {
	ErrorCode() Code
}

// Error is the shared internal error model used across packages.
type Error struct {
	Code    Code
	Op      string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	msg := e.Message
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	if msg == "" {
		msg = "operation failed"
	}

	if e.Op != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Op, msg, e.Code)
	}
	return fmt.Sprintf("%s (%s)", msg, e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *Error) ErrorCode() Code {
	if e == nil || e.Code == "" {
		return CodeInternal
	}
	return e.Code
}

// New creates a coded error without a wrapped cause.
func New(code Code, op, message string) error {
	return &Error{
		Code:    code,
		Op:      op,
		Message: message,
	}
}

// Wrap wraps an existing error with a code and operation.
func Wrap(code Code, op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Code: code,
		Op:   op,
		Err:  err,
	}
}

// CodeOf returns the shared internal code if present.
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}

	var coded Coded
	if errors.As(err, &coded) {
		return coded.ErrorCode()
	}

	return CodeInternal
}
