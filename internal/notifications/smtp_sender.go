package notifications

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	UseTLS   bool
}

type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) *SMTPSender { return &SMTPSender{cfg: cfg} }

func (s *SMTPSender) Enabled() bool {
	return strings.TrimSpace(s.cfg.Host) != "" && s.cfg.Port > 0 && strings.TrimSpace(s.cfg.From) != ""
}

func (s *SMTPSender) Send(to string, subject string, htmlBody string) error {
	if !s.Enabled() {
		return nil
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	var c *smtp.Client
	var err error

	if s.cfg.UseTLS && s.cfg.Port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.cfg.Host})
		if err != nil {
			return err
		}
		host, _, _ := net.SplitHostPort(addr)
		c, err = smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
	} else {
		c, err = smtp.Dial(addr)
		if err != nil {
			return err
		}
		if s.cfg.UseTLS {
			if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host}); err != nil {
				return err
			}
		}
	}
	defer c.Close()

	if strings.TrimSpace(s.cfg.User) != "" {
		auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
		if err := c.Auth(auth); err != nil {
			return err
		}
	}

	if err := c.Mail(s.cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	msg := buildMIMEMessage(s.cfg.From, to, subject, htmlBody)
	_, err = w.Write([]byte(msg))
	if closeErr := w.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	return c.Quit()
}

func buildMIMEMessage(from, to, subject, bodyHTML string) string {
	const crlf = "\r\n"
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}
	var b strings.Builder
	for _, h := range headers {
		b.WriteString(h)
		b.WriteString(crlf)
	}
	b.WriteString(crlf)
	b.WriteString(bodyHTML)
	return b.String()
}
