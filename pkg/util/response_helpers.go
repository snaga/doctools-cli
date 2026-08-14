package util

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"doctools-cli/pkg/models"
)

var OutputWriter io.Writer = os.Stdout

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
		ErrorCode: "EXECUTION_ERROR",
		Message:   err.Error(),
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	w := OutputWriter
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
		ErrorCode: "EXECUTION_ERROR",
		Message:   err.Error(),
		Hint:      hint,
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	w := OutputWriter
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


