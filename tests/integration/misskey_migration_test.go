//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	defaultAuthSecret       = "miwkey-importer-integration-secret"
	defaultExtensionURL     = "http://localhost:38080"
	defaultSourceMisskeyURL = "http://localhost:31200"
	defaultTargetMisskeyURL = "http://localhost:32500"
	defaultPostgresURL      = "postgres://misskey:misskey@localhost:35432/misskey?sslmode=disable"
	defaultNotifieeID       = "9yuser0001"
)

var seededNotificationIDs = []string{
	"9z00000001",
	"9z00000002",
	"9z00000003",
}

func TestMigrationImportsSeededNotifications(t *testing.T) {
	cfg := testConfig{
		repoRoot:         repoRoot(t),
		authSecret:       envOrDefault("AUTH_SECRET", defaultAuthSecret),
		extensionURL:     envOrDefault("EXTENSION_URL", defaultExtensionURL),
		sourceMisskeyURL: envOrDefault("SOURCE_MISSKEY_URL", defaultSourceMisskeyURL),
		targetMisskeyURL: envOrDefault("TARGET_MISSKEY_URL", defaultTargetMisskeyURL),
		postgresURL:      envOrDefault("INTEGRATION_POSTGRES_URL", defaultPostgresURL),
		notifieeID:       defaultNotifieeID,
		goCmd:            envOrDefault("GO_CMD", "go"),
	}

	waitHTTPStatus(t, "Misskey 12.119.0", cfg.sourceMisskeyURL+"/", http.StatusOK)
	waitHTTPStatus(t, "Misskey 2025.12.2", cfg.targetMisskeyURL+"/", http.StatusOK)
	waitHTTPStatus(t, "extension auth gate", notificationsURL(cfg.extensionURL, cfg.notifieeID, 1), http.StatusUnauthorized)

	runImporter(t, cfg, true, len(seededNotificationIDs))
	runImporter(t, cfg, false, len(seededNotificationIDs))

	verifyNotifications(t, cfg, len(seededNotificationIDs), seededNotificationIDs)
}

func TestVerifyExistingNotifications(t *testing.T) {
	cfg := testConfig{
		repoRoot:     repoRoot(t),
		authSecret:   requiredEnv(t, "AUTH_SECRET"),
		extensionURL: envOrDefault("EXTENSION_URL", defaultExtensionURL),
		notifieeID:   requiredEnv(t, "NOTIFIEE_ID"),
		goCmd:        envOrDefault("GO_CMD", "go"),
	}

	minCount := envIntOrDefault(t, "EXPECTED_MIN_COUNT", 1)
	expectedIDs := csvEnv("EXPECTED_IDS")

	waitHTTPStatus(t, "extension auth gate", notificationsURL(cfg.extensionURL, cfg.notifieeID, 1), http.StatusUnauthorized)
	verifyNotifications(t, cfg, minCount, expectedIDs)
}

type testConfig struct {
	repoRoot         string
	authSecret       string
	extensionURL     string
	sourceMisskeyURL string
	targetMisskeyURL string
	postgresURL      string
	notifieeID       string
	goCmd            string
}

func runImporter(t *testing.T, cfg testConfig, dryRun bool, limit int) {
	t.Helper()

	args := []string{
		"run", "./cmd/miwkey-notification-importer",
		"--postgres-url", cfg.postgresURL,
		"--extension-url", cfg.extensionURL,
		"--secret", cfg.authSecret,
		"--limit", strconv.Itoa(limit),
	}
	if dryRun {
		args = append(args, "--dry-run")
	}

	stdout, stderr := runCommand(t, cfg.repoRoot, cfg.goCmd, args...)
	if stdout != "" {
		t.Logf("importer stdout: %s", stdout)
	}
	t.Logf("importer stderr: %s", stderr)
}

func verifyNotifications(t *testing.T, cfg testConfig, minCount int, expectedIDs []string) {
	t.Helper()

	token := strings.TrimSpace(runCommandStdout(t, cfg.repoRoot, cfg.goCmd, "run", "./cmd/miwkey-notification-importer", "token", "--secret", cfg.authSecret))
	if token == "" {
		t.Fatal("token command returned empty token")
	}

	req, err := http.NewRequest(http.MethodGet, notificationsURL(cfg.extensionURL, cfg.notifieeID, 100), nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET notifications error = %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET notifications status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var notifications []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&notifications); err != nil {
		t.Fatalf("decode notifications error = %v", err)
	}
	if len(notifications) < minCount {
		t.Fatalf("expected at least %d notifications, got %d", minCount, len(notifications))
	}

	if len(expectedIDs) == 0 {
		return
	}

	actualIDs := make([]string, 0, len(notifications))
	for _, notification := range notifications {
		actualIDs = append(actualIDs, notification.ID)
	}
	sort.Strings(actualIDs)
	expectedSorted := append([]string(nil), expectedIDs...)
	sort.Strings(expectedSorted)

	if strings.Join(actualIDs, ",") != strings.Join(expectedSorted, ",") {
		t.Fatalf("notification ids = %v, want %v", actualIDs, expectedSorted)
	}
}

func waitHTTPStatus(t *testing.T, label string, rawURL string, expected int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Minute)
	var lastStatus int
	var lastErr error

	client := &http.Client{Timeout: 5 * time.Second}
	for time.Now().Before(deadline) {
		res, err := client.Get(rawURL)
		if err == nil {
			lastStatus = res.StatusCode
			res.Body.Close()
			if res.StatusCode == expected {
				t.Logf("%s is ready (%d)", label, expected)
				return
			}
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("%s did not return %d from %s, last status=%d, last error=%v", label, expected, rawURL, lastStatus, lastErr)
}

func runCommandStdout(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()

	stdout, _ := runCommand(t, dir, name, args...)
	return stdout
}

func runCommand(t *testing.T, dir string, name string, args ...string) (string, string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s failed: %v\nstdout:\n%s\nstderr:\n%s", name, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}

	return stdout.String(), stderr.String()
}

func notificationsURL(extensionURL string, notifieeID string, limit int) string {
	base := strings.TrimRight(extensionURL, "/")
	values := url.Values{}
	values.Set("userId", notifieeID)
	values.Set("limit", strconv.Itoa(limit))
	return base + "/api/v1/notifications?" + values.Encode()
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func requiredEnv(t *testing.T, key string) string {
	t.Helper()

	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envIntOrDefault(t *testing.T, key string, fallback int) int {
	t.Helper()

	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("%s must be an integer: %v", key, err)
	}
	return parsed
}

func csvEnv(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
