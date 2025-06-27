package common

import "fmt"

// DevStatsError represents an error in the dev-stats application
type DevStatsError struct {
	Message string
	Cause   error
}

func (e *DevStatsError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// NewError creates a new DevStatsError
func NewError(format string, args ...interface{}) *DevStatsError {
	return &DevStatsError{
		Message: fmt.Sprintf(format, args...),
	}
}

// WrapError wraps an existing error with additional context
func WrapError(cause error, format string, args ...interface{}) *DevStatsError {
	return &DevStatsError{
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}
