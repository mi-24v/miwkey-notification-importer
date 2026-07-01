package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mi-24v/miwkey-notification-importer/internal/auth"
	"github.com/mi-24v/miwkey-notification-importer/internal/notification"
)

type Client struct {
	BaseURL     string
	HTTPClient  *http.Client
	TokenSource auth.TokenSource
}

func (c Client) CreateNotification(ctx context.Context, payload notification.Payload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	token, err := c.TokenSource.Token(ctx)
	if err != nil {
		return fmt.Errorf("create auth token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.createNotificationURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post notification %s: %w", payload.ID, err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated || res.StatusCode == http.StatusNoContent {
		return nil
	}

	responseBody, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	return fmt.Errorf("post notification %s: status %d: %s", payload.ID, res.StatusCode, strings.TrimSpace(string(responseBody)))
}

func (c Client) createNotificationURL() string {
	return strings.TrimRight(c.BaseURL, "/") + "/api/v1/notifications"
}
