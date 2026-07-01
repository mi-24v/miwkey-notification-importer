# Miwkey Notification Importer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that imports old Misskey PostgreSQL `notification` rows into the miwkey-extension notification server.

**Architecture:** The CLI reads rows from restored PostgreSQL in `createdAt, id` order, maps each row to the extension server JSON payload, and posts one notification at a time. It fails on the first invalid row or HTTP error and reports the last successful id plus the failed id so a human can retry or intentionally resume past a broken row.

**Tech Stack:** Go 1.23, standard `flag` and `net/http`, `github.com/jackc/pgx/v5/pgxpool` for PostgreSQL, `github.com/golang-jwt/jwt/v5` for HS256 JWTs.

---

## File Structure

- `go.mod`: module definition and dependencies.
- `internal/notification/notification.go`: migration row, payload type, type validation, and row-to-payload mapping.
- `internal/auth/auth.go`: static bearer token and per-request HS256 JWT token generation.
- `internal/extension/client.go`: extension server HTTP client.
- `internal/postgres/reader.go`: PostgreSQL row reader and resume boundary query.
- `internal/importer/importer.go`: import loop, dry-run behavior, counters, and stop-on-first-error semantics.
- `cmd/miwkey-notification-importer/main.go`: CLI flags, validation, dependency wiring, and exit code.
- `README.md`: dump restore and importer usage.

## Task 1: Module And Notification Mapping

**Files:**
- Create: `go.mod`
- Create: `internal/notification/notification.go`
- Test: `internal/notification/notification_test.go`

- [ ] **Step 1: Initialize the Go module**

Run:

```bash
go mod init github.com/mi-24v/miwkey-notification-importer
go get github.com/golang-jwt/jwt/v5 github.com/jackc/pgx/v5/pgxpool
```

Expected: `go.mod` exists and names the module.

- [ ] **Step 2: Write failing mapping tests**

Add `internal/notification/notification_test.go` with tests that create a `Row`, call `ToPayload`, and assert:

```go
payload, err := row.ToPayload()
if err != nil {
	t.Fatalf("ToPayload() error = %v", err)
}
if payload.ID != "9m8e7d6c5b" {
	t.Fatalf("ID = %q", payload.ID)
}
if payload.Type != "follow" {
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
```

Also add a test where `Type: "pollVote"` returns an error containing `unsupported notification type`.

- [ ] **Step 3: Run tests to verify RED**

Run:

```bash
go test ./internal/notification
```

Expected: FAIL because `Row` and `ToPayload` are undefined.

- [ ] **Step 4: Implement minimal mapping**

Create `internal/notification/notification.go` with:

```go
package notification

import (
	"fmt"
	"time"
)

type Row struct {
	ID               string
	CreatedAt        time.Time
	NotifieeID       string
	NotifierID       *string
	Type             string
	IsRead           bool
	NoteID           *string
	Reaction         *string
	Choice           *int
	CustomBody       *string
	CustomHeader     *string
	CustomIcon       *string
	AppAccessTokenID *string
	Achievement      *string
}

type Payload struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"createdAt"`
	NotifieeID       string    `json:"notifieeId"`
	NotifierID       *string   `json:"notifierId,omitempty"`
	Type             string    `json:"type"`
	IsRead           bool      `json:"isRead"`
	NoteID           *string   `json:"noteId,omitempty"`
	Reaction         *string   `json:"reaction,omitempty"`
	CustomBody       *string   `json:"customBody,omitempty"`
	CustomHeader     *string   `json:"customHeader,omitempty"`
	CustomIcon       *string   `json:"customIcon,omitempty"`
	AppAccessTokenID *string   `json:"appAccessTokenId,omitempty"`
	Achievement      *string   `json:"achievement,omitempty"`
}

func (r Row) ToPayload() (Payload, error) {
	if !isSupportedType(r.Type) {
		return Payload{}, fmt.Errorf("unsupported notification type %q", r.Type)
	}
	return Payload{
		ID:               r.ID,
		CreatedAt:        r.CreatedAt,
		NotifieeID:       r.NotifieeID,
		NotifierID:       r.NotifierID,
		Type:             r.Type,
		IsRead:           r.IsRead,
		NoteID:           r.NoteID,
		Reaction:         r.Reaction,
		CustomBody:       r.CustomBody,
		CustomHeader:     r.CustomHeader,
		CustomIcon:       r.CustomIcon,
		AppAccessTokenID: r.AppAccessTokenID,
		Achievement:      r.Achievement,
	}, nil
}
```

- [ ] **Step 5: Run tests to verify GREEN**

Run:

```bash
go test ./internal/notification
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/notification
git commit -m "feat(notification): map database rows to payloads" -m "Why: Convert old Misskey notification rows into extension server requests.\n\nWhat: Add row and payload types plus supported type validation.\n\nHow: Map nullable columns with pointer fields and reject unsupported notification types.\n\nTests: go test ./internal/notification\n\nNotes: The legacy choice column is intentionally ignored."
```

## Task 2: Authentication And Extension Client

**Files:**
- Create: `internal/auth/auth.go`
- Test: `internal/auth/auth_test.go`
- Create: `internal/extension/client.go`
- Test: `internal/extension/client_test.go`

- [ ] **Step 1: Write failing auth tests**

Add tests for:

```go
source := auth.StaticBearer("abc")
token, err := source.Token(context.Background())
if err != nil {
	t.Fatalf("Token() error = %v", err)
}
if token != "abc" {
	t.Fatalf("token = %q", token)
}
```

and:

```go
source := auth.JWTSource{Secret: "secret", Now: func() time.Time {
	return time.Unix(1000, 0).UTC()
}}
token, err := source.Token(context.Background())
if err != nil {
	t.Fatalf("Token() error = %v", err)
}
if token == "" {
	t.Fatal("token is empty")
}
```

- [ ] **Step 2: Run auth tests to verify RED**

Run:

```bash
go test ./internal/auth
```

Expected: FAIL because package `internal/auth` does not exist.

- [ ] **Step 3: Implement token sources**

Create `internal/auth/auth.go` with `TokenSource`, `StaticBearer`, and `JWTSource` using `jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{...})` and `SignedString([]byte(secret))`.

- [ ] **Step 4: Run auth tests to verify GREEN**

Run:

```bash
go test ./internal/auth
```

Expected: PASS.

- [ ] **Step 5: Write failing extension client tests**

Use `httptest.Server`, `auth.StaticBearer("token")`, and `notification.Payload`. Assert the request path is `/api/v1/notifications`, authorization is `Bearer token`, method is `POST`, and a 201 response returns nil error. Add one 500 response test that returns an error containing status `500`.

- [ ] **Step 6: Run extension tests to verify RED**

Run:

```bash
go test ./internal/extension
```

Expected: FAIL because package `internal/extension` does not exist.

- [ ] **Step 7: Implement HTTP client**

Create `internal/extension/client.go` with:

```go
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	TokenSource auth.TokenSource
}

func (c Client) CreateNotification(ctx context.Context, payload notification.Payload) error
```

The method marshals JSON, obtains a token per request, posts to `{BaseURL}/api/v1/notifications`, treats 200/201/204 as success, and returns an error for all other status codes.

- [ ] **Step 8: Run extension tests to verify GREEN**

Run:

```bash
go test ./internal/auth ./internal/extension
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/auth internal/extension
git commit -m "feat(extension): post notifications with bearer auth" -m "Why: Send imported notifications to the extension server securely.\n\nWhat: Add static bearer and JWT token sources plus an HTTP client for notification creation.\n\nHow: Generate tokens per request and POST JSON to /api/v1/notifications.\n\nTests: go test ./internal/auth ./internal/extension\n\nNotes: Non-2xx responses stop the import."
```

## Task 3: PostgreSQL Reader

**Files:**
- Create: `internal/postgres/reader.go`
- Test: `internal/postgres/reader_test.go`

- [ ] **Step 1: Write failing query builder tests**

Add tests for `BuildListQuery` with no resume id and with a resume boundary. Assert the SQL contains `ORDER BY "createdAt", id`, includes `LIMIT`, and uses `WHERE ("createdAt", id) > ($1, $2)` when resuming.

- [ ] **Step 2: Run tests to verify RED**

Run:

```bash
go test ./internal/postgres
```

Expected: FAIL because package `internal/postgres` does not exist.

- [ ] **Step 3: Implement reader and query builder**

Create `internal/postgres/reader.go` with:

```go
type ResumeBoundary struct {
	CreatedAt time.Time
	ID        string
}

func BuildListQuery(limit int, resume *ResumeBoundary) (string, []any)
```

Also define `Reader` backed by a `DB` interface with `Query` and `QueryRow`, scan rows into `notification.Row`, call `rows.Close()`, and check `rows.Err()`.

- [ ] **Step 4: Run tests to verify GREEN**

Run:

```bash
go test ./internal/postgres
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/postgres
git commit -m "feat(postgres): read notifications in stable order" -m "Why: Import notifications from a restored PostgreSQL dump.\n\nWhat: Add stable list query construction and reader scaffolding.\n\nHow: Order by createdAt and id, and resume using a createdAt/id boundary.\n\nTests: go test ./internal/postgres\n\nNotes: The reader checks rows.Err after iteration."
```

## Task 4: Import Orchestration And CLI

**Files:**
- Create: `internal/importer/importer.go`
- Test: `internal/importer/importer_test.go`
- Create: `cmd/miwkey-notification-importer/main.go`

- [ ] **Step 1: Write failing importer tests**

Use fake reader and fake extension client. Assert:

```go
result, err := importer.Run(ctx, importer.Options{DryRun: true})
if err != nil {
	t.Fatalf("Run() error = %v", err)
}
if result.SuccessCount != 2 {
	t.Fatalf("SuccessCount = %d", result.SuccessCount)
}
if fakeClient.Calls != 0 {
	t.Fatalf("fakeClient.Calls = %d", fakeClient.Calls)
}
```

Add a second test where the second row fails and the result contains `LastSuccessfulID` for the first row and `FailedID` for the second row.

- [ ] **Step 2: Run tests to verify RED**

Run:

```bash
go test ./internal/importer
```

Expected: FAIL because package `internal/importer` does not exist.

- [ ] **Step 3: Implement importer loop**

Create `internal/importer/importer.go` with interfaces:

```go
type Reader interface {
	Read(ctx context.Context, opts ReadOptions) ([]notification.Row, error)
}

type ExtensionClient interface {
	CreateNotification(ctx context.Context, payload notification.Payload) error
}
```

Implement stop-on-first-error behavior and result counters.

- [ ] **Step 4: Run importer tests to verify GREEN**

Run:

```bash
go test ./internal/importer
```

Expected: PASS.

- [ ] **Step 5: Implement CLI**

Create `cmd/miwkey-notification-importer/main.go` using `flag` for `--postgres-url`, `--extension-url`, `--bearer-token`, `--secret`, `--dry-run`, `--limit`, `--resume-after-id`, and `--batch-size`. Validate exactly one auth option is present, wire pgxpool, auth source, extension client, reader, and importer.

- [ ] **Step 6: Run package tests and build**

Run:

```bash
go test ./...
go build ./cmd/miwkey-notification-importer
```

Expected: PASS and successful build.

- [ ] **Step 7: Commit**

```bash
git add internal/importer cmd/miwkey-notification-importer
git commit -m "feat(importer): add migration command" -m "Why: Provide the executable migration path from restored PostgreSQL to the extension server.\n\nWhat: Add import orchestration and CLI wiring.\n\nHow: Validate flags, read rows, map payloads, post notifications, and stop on first failure.\n\nTests: go test ./... && go build ./cmd/miwkey-notification-importer\n\nNotes: Dry-run validates payloads without HTTP requests."
```

## Task 5: README And Final Verification

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

Document:

```markdown
# miwkey-notification-importer

One-shot importer for migrating old Misskey PostgreSQL notification rows into miwkey-extension.
```

Include restore, dry-run, import, and resume examples from the spec.

- [ ] **Step 2: Run final verification**

Run:

```bash
gofmt -w .
go test ./...
go build ./cmd/miwkey-notification-importer
git status --short
```

Expected: tests pass, build succeeds, and only intended files are modified.

- [ ] **Step 3: Commit**

```bash
git add README.md go.mod go.sum cmd internal docs
git commit -m "docs: add migration importer usage" -m "Why: Make the one-shot migration process repeatable.\n\nWhat: Document dump restore, dry-run, import, and resume commands.\n\nHow: Add README usage examples matching the implemented flags.\n\nTests: go test ./... && go build ./cmd/miwkey-notification-importer\n\nNotes: Importer intentionally does not parse raw dump.sql."
```
