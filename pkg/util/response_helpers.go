package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"doctools-cli/pkg/models"
)

var OutputWriter io.Writer = os.Stdout
var ErrorWriter io.Writer = os.Stderr

// DetermineErrorCode classifies an error into a standardized error code.
func DetermineErrorCode(err error) string {
	if err == nil {
		return "EXECUTION_ERROR"
	}
	if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
		return "FILE_NOT_FOUND"
	}
	if errors.Is(err, os.ErrPermission) {
		return "PERMISSION_DENIED"
	}
	if errors.Is(err, os.ErrInvalid) {
		return "INVALID_ARGUMENT"
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no such file") || strings.Contains(msg, "does not exist") {
		return "FILE_NOT_FOUND"
	}
	if strings.Contains(msg, "invalid") || strings.Contains(msg, "out of bounds") || strings.Contains(msg, "out of range") || strings.Contains(msg, "empty") || strings.Contains(msg, "unsupported") || strings.Contains(msg, "required") {
		return "INVALID_ARGUMENT"
	}
	return "EXECUTION_ERROR"
}

// PrintJSONResponse prints formatted success JSON response to OutputWriter.
func PrintJSONResponse(resp models.SuccessResponse) {
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		ExitWithError(err)
	}
	w := OutputWriter
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, string(b))
}

var DefaultExitWithErrorImpl = func(err error) {
	resp := models.ErrorResponse{
		Status:    "error",
		ErrorCode: DetermineErrorCode(err),
		Message:   err.Error(),
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	w := ErrorWriter
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintln(w, string(b))
	os.Exit(1)
}

var ExitWithErrorHook func(err error) = DefaultExitWithErrorImpl

// ExitWithError prints error response and exits.
func ExitWithError(err error) {
	ExitWithErrorHook(err)
}

var DefaultExitWithErrorWithHintImpl = func(err error, hint string) {
	resp := models.ErrorResponse{
		Status:    "error",
		ErrorCode: DetermineErrorCode(err),
		Message:   err.Error(),
		Hint:      hint,
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	w := ErrorWriter
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintln(w, string(b))
	os.Exit(1)
}

var ExitWithErrorWithHintHook func(err error, hint string) = DefaultExitWithErrorWithHintImpl

// ExitWithErrorWithHint prints error response with a hint and exits.
func ExitWithErrorWithHint(err error, hint string) {
	ExitWithErrorWithHintHook(err, hint)
}

// HandleError is an alias for ExitWithError.
func HandleError(err error) {
	ExitWithError(err)
}



