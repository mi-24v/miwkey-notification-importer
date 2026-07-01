package notification_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mi-24v/miwkey-notification-importer/internal/notification"
)

func TestRowToPayloadMapsNullableFields(t *testing.T) {
	createdAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	notifierID := "notifier"
	reaction := ":smile:"
	customHeader := "header"

	row := notification.Row{
		ID:           "9m8e7d6c5b",
		CreatedAt:    createdAt,
		NotifieeID:   "notifiee",
		NotifierID:   &notifierID,
		Type:         "reaction",
		IsRead:       true,
		Reaction:     &reaction,
		CustomHeader: &customHeader,
	}

	payload, err := row.ToPayload()
	if err != nil {
		t.Fatalf("ToPayload() error = %v", err)
	}

	if payload.ID != "9m8e7d6c5b" {
		t.Fatalf("ID = %q", payload.ID)
	}
	if payload.CreatedAt != createdAt {
		t.Fatalf("CreatedAt = %s", payload.CreatedAt)
	}
	if payload.Type != "reaction" {
		t.Fatalf("Type = %q", payload.Type)
	}
	if payload.NotifieeID != "notifiee" {
		t.Fatalf("NotifieeID = %q", payload.NotifieeID)
	}
	if payload.NotifierID == nil || *payload.NotifierID != "notifier" {
		t.Fatalf("NotifierID = %v", payload.NotifierID)
	}
	if payload.NoteID != nil {
		t.Fatalf("NoteID = %v", payload.NoteID)
	}
	if payload.Reaction == nil || *payload.Reaction != ":smile:" {
		t.Fatalf("Reaction = %v", payload.Reaction)
	}
	if payload.CustomHeader == nil || *payload.CustomHeader != "header" {
		t.Fatalf("CustomHeader = %v", payload.CustomHeader)
	}
	if !payload.IsRead {
		t.Fatal("IsRead = false")
	}
}

func TestRowToPayloadRejectsUnsupportedType(t *testing.T) {
	row := notification.Row{
		ID:         "bad",
		CreatedAt:  time.Now().UTC(),
		NotifieeID: "notifiee",
		Type:       "pollVote",
	}

	_, err := row.ToPayload()
	if err == nil {
		t.Fatal("ToPayload() error = nil")
	}
	if !strings.Contains(err.Error(), "unsupported notification type") {
		t.Fatalf("error = %v", err)
	}
}
