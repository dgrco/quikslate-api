package handler

type SimpleResponse struct {
	Message string `json:"message"`
}

const (
	DEFAULT_MAX_REQUEST_BODY_SIZE = 1 * 1024 * 1024
)
