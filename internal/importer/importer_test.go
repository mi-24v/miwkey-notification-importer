package importer_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mi-24v/miwkey-notification-importer/internal/importer"
	"github.com/mi-24v/miwkey-notification-importer/internal/notification"
	"github.com/mi-24v/miwkey-notification-importer/internal/postgres"
)

func TestImporterDryRunDoesNotPostNotifications(t *testing.T) {
	reader := &fakeReader{rows: []notification.Row{
		notificationRow("first", "follow"),
		notificationRow("second", "reaction"),
	}}
	client := &fakeClient{}
	imp := importer.Importer{Reader: reader, Client: client}

	result, err := imp.Run(context.Background(), importer.Options{DryRun: true, BatchSize: 10})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.SuccessCount != 2 {
		t.Fatalf("SuccessCount = %d", result.SuccessCount)
	}
	if result.FailureCount != 0 {
		t.Fatalf("FailureCount = %d", result.FailureCount)
	}
	if result.LastSuccessfulID != "second" {
		t.Fatalf("LastSuccessfulID = %q", result.LastSuccessfulID)
	}
	if client.calls != 0 {
		t.Fatalf("client calls = %d", client.calls)
	}
}

func TestImporterStopsOnFirstHTTPFailure(t *testing.T) {
	reader := &fakeReader{rows: []notification.Row{
		notificationRow("first", "follow"),
		notificationRow("second", "reaction"),
		notificationRow("third", "follow"),
	}}
	client := &fakeClient{failID: "second"}
	imp := importer.Importer{Reader: reader, Client: client}

	result, err := imp.Run(context.Background(), importer.Options{BatchSize: 10})
	if err == nil {
		t.Fatal("Run() error = nil")
	}
	if !strings.Contains(err.Error(), "second") {
		t.Fatalf("error = %v", err)
	}
	if result.SuccessCount != 1 {
		t.Fatalf("SuccessCount = %d", result.SuccessCount)
	}
	if result.FailureCount != 1 {
		t.Fatalf("FailureCount = %d", result.FailureCount)
	}
	if result.LastSuccessfulID != "first" {
		t.Fatalf("LastSuccessfulID = %q", result.LastSuccessfulID)
	}
	if result.FailedID != "second" {
		t.Fatalf("FailedID = %q", result.FailedID)
	}
	if client.calls != 2 {
		t.Fatalf("client calls = %d", client.calls)
	}
}

func TestImporterStopsOnPayloadValidationFailure(t *testing.T) {
	reader := &fakeReader{rows: []notification.Row{
		notificationRow("first", "follow"),
		notificationRow("bad", "pollVote"),
	}}
	client := &fakeClient{}
	imp := importer.Importer{Reader: reader, Client: client}

	result, err := imp.Run(context.Background(), importer.Options{DryRun: true, BatchSize: 10})
	if err == nil {
		t.Fatal("Run() error = nil")
	}
	if !strings.Contains(err.Error(), "unsupported notification type") {
		t.Fatalf("error = %v", err)
	}
	if result.LastSuccessfulID != "first" {
		t.Fatalf("LastSuccessfulID = %q", result.LastSuccessfulID)
	}
	if result.FailedID != "bad" {
		t.Fatalf("FailedID = %q", result.FailedID)
	}
}

func TestImporterSkipsExcludedUnsupportedTypes(t *testing.T) {
	reader := &fakeReader{rows: []notification.Row{
		notificationRow("first", "follow"),
		notificationRow("bad", "pollVote"),
		notificationRow("second", "reaction"),
	}}
	client := &fakeClient{}
	imp := importer.Importer{Reader: reader, Client: client}

	result, err := imp.Run(context.Background(), importer.Options{
		DryRun:       true,
		BatchSize:    10,
		ExcludeTypes: []string{"pollVote"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.SuccessCount != 2 {
		t.Fatalf("SuccessCount = %d", result.SuccessCount)
	}
	if result.SkippedCount != 1 {
		t.Fatalf("SkippedCount = %d", result.SkippedCount)
	}
	if result.FailureCount != 0 {
		t.Fatalf("FailureCount = %d", result.FailureCount)
	}
	if result.LastSuccessfulID != "second" {
		t.Fatalf("LastSuccessfulID = %q", result.LastSuccessfulID)
	}
	if result.LastSkippedID != "bad" {
		t.Fatalf("LastSkippedID = %q", result.LastSkippedID)
	}
}

func notificationRow(id string, notificationType string) notification.Row {
	return notification.Row{
		ID:         id,
		CreatedAt:  time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		NotifieeID: "notifiee",
		Type:       notificationType,
	}
}

type fakeReader struct {
	rows  []notification.Row
	calls int
}

func (r *fakeReader) Read(_ context.Context, _ postgres.ReadOptions) ([]notification.Row, error) {
	r.calls++
	if r.calls > 1 {
		return nil, nil
	}
	return r.rows, nil
}

type fakeClient struct {
	failID string
	calls  int
}

func (c *fakeClient) CreateNotification(_ context.Context, payload notification.Payload) error {
	c.calls++
	if payload.ID == c.failID {
		return errors.New("post failed")
	}
	return nil
}
