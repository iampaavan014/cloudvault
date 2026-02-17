package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
)

// Notifier defines the interface for sending alerts
type Notifier interface {
	SendAlert(title, message string) error
}

// SlackNotifier sends alerts to Slack webhooks
type SlackNotifier struct {
	WebhookURL string
}

func (s *SlackNotifier) SendAlert(title, message string) error {
	slog.Info("Sending Slack Alert", "title", title)

	payload := map[string]interface{}{
		"text": fmt.Sprintf("*%s*\n%s", title, message),
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(s.WebhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("slack notification failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned non-200 status: %d", resp.StatusCode)
	}

	return nil
}

// EmailNotifier sends alerts via SMTP
type EmailNotifier struct {
	SMTPServer string
	SMTPPort   string
	Username   string
	Password   string
	From       string
	To         string
}

func (e *EmailNotifier) SendAlert(title, message string) error {
	slog.Info("Sending Email Alert", "title", title, "to", e.To)

	auth := smtp.PlainAuth("", e.Username, e.Password, e.SMTPServer)
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", e.To, title, message))

	addr := fmt.Sprintf("%s:%s", e.SMTPServer, e.SMTPPort)
	err := smtp.SendMail(addr, auth, e.From, []string{e.To}, msg)
	if err != nil {
		return fmt.Errorf("email notification failed: %w", err)
	}

	return nil
}

// MultiNotifier sends alerts to multiple destinations
type MultiNotifier struct {
	Notifiers []Notifier
}

func (m *MultiNotifier) SendAlert(title, message string) error {
	for _, n := range m.Notifiers {
		if err := n.SendAlert(title, message); err != nil {
			fmt.Printf("failed to send notification: %v\n", err)
		}
	}
	return nil
}
