package ai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAIServiceHealthy(t *testing.T) {
	_ = IsAIServiceHealthy() // just confirm it doesn't panic
}

func TestGetAIServiceStatus(t *testing.T) {
	status := GetAIServiceStatus()
	assert.NotEmpty(t, status.URL)
}

func TestCheckAIServiceHealth_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	SetAIServiceURL(srv.URL)
	err := CheckAIServiceHealth()
	assert.NoError(t, err)
	// Do NOT assert IsAIServiceHealthy() — that is updated only by the background goroutine
}

func TestCheckAIServiceHealth_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	SetAIServiceURL(srv.URL)
	err := CheckAIServiceHealth()
	assert.Error(t, err)
}

func TestCheckAIServiceHealth_Unreachable(t *testing.T) {
	SetAIServiceURL("http://127.0.0.1:19999")
	err := CheckAIServiceHealth()
	assert.Error(t, err)
}
