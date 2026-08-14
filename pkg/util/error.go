package util

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"doctools-cli/pkg/models"
)

// FormatErrorJSON creates a formatted JSON byte slice for an error.
func FormatErrorJSON(errCode string, msg string, hint string) ([]byte, error) {
	resp := models.ErrorResponse{
		Status:    "error",
		ErrorCode: errCode,
		Message:   msg,
		Hint:      hint,
	}
	return json.MarshalIndent(resp, "", "  ")
}

var formatErrorJSON = FormatErrorJSON

// PrintErrorTo writes formatted error (JSON or text) to the given writer (e.g. os.Stderr).
func PrintErrorTo(w io.Writer, errCode string, msg string, hint string, isJSON bool) {
	if isJSON {
		bytes, err := formatErrorJSON(errCode, msg, hint)
		if err != nil {
			fmt.Fprintf(w, `{"status":"error","error_code":"INTERNAL_ERROR","message":%q}`, err.Error())
			return
		}
		fmt.Fprintln(w, string(bytes))
	} else {
		fmt.Fprintf(w, "Error [%s]: %s\n", errCode, msg)
		if hint != "" {
			fmt.Fprintf(w, "Hint: %s\n", hint)
		}
	}
}

// PrintError writes formatted error to os.Stderr.
func PrintError(errCode string, msg string, hint string, isJSON bool) {
	PrintErrorTo(os.Stderr, errCode, msg, hint, isJSON)
}

// FormatSuccessJSON creates a formatted JSON byte slice for a success response.
func FormatSuccessJSON(data interface{}) ([]byte, error) {
	resp := models.Response{
		Status: "success",
		Data:   data,
	}
	return json.MarshalIndent(resp, "", "  ")
}

// PrintSuccessTo writes formatted success response (JSON or text) to the given writer.
func PrintSuccessTo(w io.Writer, data interface{}, isJSON bool) {
	if isJSON {
		bytes, err := FormatSuccessJSON(data)
		if err != nil {
			PrintErrorTo(w, "JSON_ENCODE_ERROR", err.Error(), "", true)
			return
		}
		fmt.Fprintln(w, string(bytes))
	}
}

// PrintSuccess writes formatted success response to os.Stdout.
func PrintSuccess(data interface{}, isJSON bool) {
	PrintSuccessTo(os.Stdout, data, isJSON)
}
