package handler

// This package contains types that are used across files

// Response Structures

type SimpleResponse struct {
	Message string `json:"message"`
}

const (
	DEFAULT_MAX_REQUEST_BODY_SIZE = 1 * 1024 * 1024 // 1 MB
)
