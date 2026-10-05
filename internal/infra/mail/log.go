package mail

import (
	"fmt"
	"log"

	"github.com/dgrco/quikslate/internal/domain"
)

// LoggerMailer is development-only: a reset link is a live credential, and
// this writes it to the log in plaintext for anyone with log access.
type LoggerMailer struct {
	frontendBaseURL string
}

func NewLoggerMailer(frontendBaseURL string) *LoggerMailer {
	return &LoggerMailer{frontendBaseURL}
}

func (m *LoggerMailer) SendForgotPassword(to, token string) error {
	frontendURL := fmt.Sprintf("%s/password-reset/%s", m.frontendBaseURL, token)
	log.Printf("[SendForgotPassword to=%q reset-url=%q]", to, frontendURL)
	return nil
}

var _ domain.Mailer = &LoggerMailer{}
