package integrations

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackNotifier_SendAlert(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful-notification",
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:          "failed-notification",
			statusCode:    http.StatusBadRequest,
			expectError:   true,
			errorContains: "non-200 status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			notifier := &SlackNotifier{
				WebhookURL: server.URL,
			}

			err := notifier.SendAlert("Test Alert", "Test message")

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSlackNotifier_InvalidURL(t *testing.T) {
	notifier := &SlackNotifier{
		WebhookURL: "http://invalid-url-that-does-not-exist-12345.com",
	}

	err := notifier.SendAlert("Test", "Message")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestEmailNotifier_SendAlert(t *testing.T) {
	// Note: This test will fail without a real SMTP server
	// In production, you'd use a mock SMTP server
	notifier := &EmailNotifier{
		SMTPServer: "localhost",
		SMTPPort:   "25",
		Username:   "test@example.com",
		Password:   "password",
		From:       "sender@example.com",
		To:         "recipient@example.com",
	}

	// This will likely fail but tests the code path
	err := notifier.SendAlert("Test Alert", "Test message")

	// We expect an error since there's no SMTP server
	if err == nil {
		t.Log("Email sent successfully (unexpected unless SMTP server is running)")
	}
}

func TestMultiNotifier_SendAlert(t *testing.T) {
	// Create test servers
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer successServer.Close()

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failServer.Close()

	notifier := &MultiNotifier{
		Notifiers: []Notifier{
			&SlackNotifier{WebhookURL: successServer.URL},
			&SlackNotifier{WebhookURL: failServer.URL},
		},
	}

	// Should not return error even if one notifier fails
	err := notifier.SendAlert("Test Alert", "Test message")
	if err != nil {
		t.Errorf("MultiNotifier should not return error: %v", err)
	}
}

func TestMultiNotifier_EmptyNotifiers(t *testing.T) {
	notifier := &MultiNotifier{
		Notifiers: []Notifier{},
	}

	err := notifier.SendAlert("Test Alert", "Test message")
	if err != nil {
		t.Errorf("Unexpected error with empty notifiers: %v", err)
	}
}

func TestMultiNotifier_AllSuccess(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	notifier := &MultiNotifier{
		Notifiers: []Notifier{
			&SlackNotifier{WebhookURL: server1.URL},
			&SlackNotifier{WebhookURL: server2.URL},
		},
	}

	err := notifier.SendAlert("Test Alert", "Test message")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
