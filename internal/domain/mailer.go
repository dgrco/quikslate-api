package domain

type Mailer interface {
	SendForgotPassword(to, token string) error
}
