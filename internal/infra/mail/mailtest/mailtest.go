package mailtest

import "github.com/dgrco/quikslate/internal/domain"

type SentMail struct {
	Op            string
	ReceiverEmail string
	Token         string
}

type Mailer struct {
	sentMail []SentMail
}

func NewMailer() *Mailer {
	return &Mailer{}
}

func (mailer *Mailer) SendForgotPassword(to, token string) error {
	mailer.sentMail = append(mailer.sentMail, SentMail{Op: "SendForgotPassword", ReceiverEmail: to, Token: token})
	return nil
}

func (mailer *Mailer) Find(op string) *SentMail {
	for i := range mailer.sentMail {
		if mailer.sentMail[i].Op == op {
			return &mailer.sentMail[i]
		}
	}
	return nil
}

func (mailer *Mailer) DidSendMail() bool {
	return len(mailer.sentMail) > 0
}

var _ domain.Mailer = &Mailer{}
