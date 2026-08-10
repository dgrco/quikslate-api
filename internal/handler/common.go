package handler

// This file holds response types and constants shared across the handler
// package: the generic SimpleResponse body and request-size limits.

// Response Structures

type SimpleResponse struct {
	Message string `json:"message"`
}

const (
	DEFAULT_MAX_REQUEST_BODY_SIZE = 1 * 1024 * 1024 // 1 MB
)
