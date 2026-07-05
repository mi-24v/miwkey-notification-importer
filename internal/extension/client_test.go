package extension_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mi-24v/miwkey-notification-importer/internal/auth"
	"github.com/mi-24v/miwkey-notification-importer/internal/extension"
	"github.com/mi-24v/miwkey-notification-importer/internal/notification"
)

func TestClientCreateNotificationPostsPayload(t *testing.T) {
	createdAt := time.Date(2021, 2, 3, 4, 5, 6, 0, time.UTC)
	var gotPayload notification.Payload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/notifications" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := extension.Client{
		BaseURL:     server.URL,
		HTTPClient:  server.Client(),
		TokenSource: auth.StaticBearer("token"),
	}

	err := client.CreateNotification(context.Background(), notification.Payload{
		ID:         "notification-id",
		CreatedAt:  createdAt,
		NotifieeID: "notifiee",
		Type:       "follow",
	})
	if err != nil {
		t.Fatalf("CreateNotification() error = %v", err)
	}

	if gotPayload.ID != "notification-id" {
		t.Fatalf("payload ID = %q", gotPayload.ID)
	}
	if !gotPayload.CreatedAt.Equal(createdAt) {
		t.Fatalf("payload CreatedAt = %s", gotPayload.CreatedAt)
	}
}

func TestClientCreateNotificationReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "broken", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := extension.Client{
		BaseURL:     server.URL,
		HTTPClient:  server.Client(),
		TokenSource: auth.StaticBearer("token"),
	}

	err := client.CreateNotification(context.Background(), notification.Payload{
		ID:         "notification-id",
		CreatedAt:  time.Now().UTC(),
		NotifieeID: "notifiee",
		Type:       "follow",
	})
	if err == nil {
		t.Fatal("CreateNotification() error = nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %v", err)
	}
}
