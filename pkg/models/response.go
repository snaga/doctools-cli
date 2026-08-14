package models

// Response represents a standard JSON response envelope for successful CLI execution.
type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
}

// SuccessResponse is an alias for Response.
type SuccessResponse = Response

// ErrorResponse represents a standard JSON error response output to stderr when CLI fails.
type ErrorResponse struct {
	Status    string `json:"status"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
	Hint      string `json:"hint,omitempty"`
}
