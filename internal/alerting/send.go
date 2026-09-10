package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"
)

type SMTPConfig struct {
	Host, Port, User, Password, From string
}

// SendEmail uses plain net/smtp. MailHog locally accepts anything; SES in
// stage/prod requires credentials, which arrive via env from Secrets Manager.
func SendEmail(cfg SMTPConfig, to, subject, body string) error {
	addr := cfg.Host + ":" + cfg.Port
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		cfg.From, to, subject, body))

	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}
	return smtp.SendMail(addr, auth, cfg.From, []string{to}, msg)
}

// SendSlack posts a message to an incoming webhook. Timeout is deliberate: a
// hanging webhook must not stall the delivery loop.
func SendSlack(ctx context.Context, webhookURL, text string) error {
	payload, _ := json.Marshal(map[string]string{"text": text})

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned %d", resp.StatusCode)
	}
	return nil
}
