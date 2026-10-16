package mailer

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type Log struct{}

func (Log) Send(_ context.Context, to, subject, body string) error {
	log.Printf("mail to=%s subject=%s body_len=%d", to, subject, len(body))
	return nil
}

type SMTP struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func (s SMTP) Send(_ context.Context, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.From, to, subject, body))
	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Pass, s.Host)
	}
	return smtp.SendMail(addr, auth, s.From, []string{to}, msg)
}
