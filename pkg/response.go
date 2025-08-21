// Package pkg provides shared types and utilities for the Bonsai API.
package pkg

// Response represents a standard API response.
type Response struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// NewResponse creates a new Response with the given code, data, and message.
func NewResponse(code int, data any, message string) Response {
	return Response{
		Code:    code,
		Data:    data,
		Message: message,
	}
}
