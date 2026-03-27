package errors

import (
	"fmt"
	"runtime"
)

// ErrorCode represents a specific error type
type ErrorCode string

const (
	// Configuration errors
	ErrCodeConfigLoad     ErrorCode = "CONFIG_LOAD_ERROR"
	ErrCodeConfigParse    ErrorCode = "CONFIG_PARSE_ERROR"
	ErrCodeConfigValidate ErrorCode = "CONFIG_VALIDATE_ERROR"

	// LLM errors
	ErrCodeLLMConnection ErrorCode = "LLM_CONNECTION_ERROR"
	ErrCodeLLMTimeout    ErrorCode = "LLM_TIMEOUT_ERROR"
	ErrCodeLLMRateLimit  ErrorCode = "LLM_RATE_LIMIT_ERROR"
	ErrCodeLLMInvalidKey ErrorCode = "LLM_INVALID_KEY_ERROR"

	// Tool errors
	ErrCodeToolExecution  ErrorCode = "TOOL_EXECUTION_ERROR"
	ErrCodeToolPermission ErrorCode = "TOOL_PERMISSION_ERROR"
	ErrCodeToolTimeout    ErrorCode = "TOOL_TIMEOUT_ERROR"

	// File errors
	ErrCodeFileNotFound   ErrorCode = "FILE_NOT_FOUND"
	ErrCodeFileRead       ErrorCode = "FILE_READ_ERROR"
	ErrCodeFileWrite      ErrorCode = "FILE_WRITE_ERROR"
	ErrCodeFilePermission ErrorCode = "FILE_PERMISSION_ERROR"

	// Agent errors
	ErrCodeAgentLoop     ErrorCode = "AGENT_LOOP_ERROR"
	ErrCodeAgentMaxSteps ErrorCode = "AGENT_MAX_STEPS_ERROR"

	// Session errors
	ErrCodeSessionCreate ErrorCode = "SESSION_CREATE_ERROR"
	ErrCodeSessionLoad   ErrorCode = "SESSION_LOAD_ERROR"
	ErrCodeSessionSave   ErrorCode = "SESSION_SAVE_ERROR"
)

// YCodeError represents a structured error with context
type YCodeError struct {
	Code     ErrorCode
	Message  string
	Err      error
	Context  map[string]interface{}
	Location string
}

// Error implements the error interface
func (e *YCodeError) Error() string {
	msg := fmt.Sprintf("[%s] %s", e.Code, e.Message)
	if e.Location != "" {
		msg += fmt.Sprintf(" (at %s)", e.Location)
	}
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	return msg
}

// Unwrap returns the wrapped error
func (e *YCodeError) Unwrap() error {
	return e.Err
}

// New creates a new YCodeError
func New(code ErrorCode, message string) *YCodeError {
	return &YCodeError{
		Code:     code,
		Message:  message,
		Location: getCallerLocation(),
	}
}

// Wrap wraps an existing error with additional context
func Wrap(code ErrorCode, message string, err error) *YCodeError {
	return &YCodeError{
		Code:     code,
		Message:  message,
		Err:      err,
		Location: getCallerLocation(),
	}
}

// WithContext adds context to an error
func (e *YCodeError) WithContext(key string, value interface{}) *YCodeError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// getCallerLocation returns the file and line number of the caller
func getCallerLocation() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// IsErrorCode checks if an error is a specific error code
func IsErrorCode(err error, code ErrorCode) bool {
	if yErr, ok := err.(*YCodeError); ok {
		return yErr.Code == code
	}
	return false
}

// GetErrorCode returns the error code if it's a YCodeError
func GetErrorCode(err error) ErrorCode {
	if yErr, ok := err.(*YCodeError); ok {
		return yErr.Code
	}
	return ""
}
