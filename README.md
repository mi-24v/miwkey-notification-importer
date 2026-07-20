# miwkey-notification-importer

One-shot importer for migrating old Misskey PostgreSQL `notification` rows into
the miwkey-extension notification server.

The importer does not parse raw `dump.sql`. Restore the dump into a temporary
PostgreSQL database first, then let the importer read the restored
`notification` table directly.

## Build

```bash
go build -o miwkey-notification-importer ./cmd/miwkey-notification-importer
```

## Migration Flow

Take the regular Misskey database dump.

```bash
pg_dump -h example.rds.amazonaws.com -U postgres -d misskey -f dump.sql
```

Restore it into a temporary PostgreSQL database.

```bash
psql "$TEMP_POSTGRES_URL" -f dump.sql
```

Run a dry-run first. This reads rows and validates payloads, but does not send
HTTP requests to the extension server.

```bash
./miwkey-notification-importer \
  --postgres-url "$TEMP_POSTGRES_URL" \
  --extension-url "$EXTENSION_URL" \
  --secret "$NOTIFICATION_EXTENSION_SECRET" \
  --dry-run
```

Run the import.

```bash
./miwkey-notification-importer \
  --postgres-url "$TEMP_POSTGRES_URL" \
  --extension-url "$EXTENSION_URL" \
  --secret "$NOTIFICATION_EXTENSION_SECRET"
```

If you intentionally do not migrate a notification type, exclude it explicitly.
Excluded rows are skipped before payload validation and counted in the result.

```bash
./miwkey-notification-importer \
  --postgres-url "$TEMP_POSTGRES_URL" \
  --extension-url "$EXTENSION_URL" \
  --secret "$NOTIFICATION_EXTENSION_SECRET" \
  --exclude-types pollVote
```

If an error stops the import, inspect the failed notification. Resume after the
last successful id.

```bash
./miwkey-notification-importer \
  --postgres-url "$TEMP_POSTGRES_URL" \
  --extension-url "$EXTENSION_URL" \
  --secret "$NOTIFICATION_EXTENSION_SECRET" \
  --resume-after-id "$LAST_SUCCESSFUL_ID"
```

Only skip a broken record intentionally. To do that, resume after the failed id.

```bash
./miwkey-notification-importer \
  --postgres-url "$TEMP_POSTGRES_URL" \
  --extension-url "$EXTENSION_URL" \
  --secret "$NOTIFICATION_EXTENSION_SECRET" \
  --resume-after-id "$FAILED_ID"
```

## Options

- `--postgres-url`: restored PostgreSQL connection URL. Required.
- `--extension-url`: notification extension server base URL. Required.
- `--secret`: shared secret used to generate short-lived HS256 JWTs.
- `--bearer-token`: pre-generated bearer token.
- `--dry-run`: validate rows without sending HTTP requests.
- `--limit`: maximum number of notifications to process.
- `--resume-after-id`: resume after the specified notification id.
- `--batch-size`: rows to read per PostgreSQL query. Default: `500`.
- `--exclude-types`: comma-separated notification types to skip before
  validation, for example `pollVote`.

Exactly one of `--secret` or `--bearer-token` is required.

## Token Helper

Generate a bearer token for manual extension API checks.

```bash
miwkey-notification-importer token --secret "$NOTIFICATION_EXTENSION_SECRET"
```

## Failure Behavior

The importer stops on the first invalid row or HTTP error. It prints:

- success count
- skipped count
- failure count
- last successful notification id
- last skipped notification id
- failed notification id

It never silently skips failed records. Only types passed with `--exclude-types`
are skipped.

## Integration Test

`mise run test:integration` creates isolated Docker services for:

- Misskey 12.119.0 with its own PostgreSQL and Redis
- Misskey 2025.12.2 with its own PostgreSQL and Redis
- DynamoDB Local
- the miwkey-extension server from `../miwkey-extension`

The task runs both Misskey migrations, seeds three 12.119.0 notification rows,
imports them through this CLI, verifies the migrated IDs through the extension
server API, and cleans up the Docker environment.

Requirements: Docker Compose and mise.

```bash
mise run test:integration
```

Set `MIWKEY_EXTENSION_DIR` if the extension server repository is not at
`../miwkey-extension`.

For manual checks against an existing extension environment:

```bash
AUTH_SECRET=... \
NOTIFIEE_ID=... \
EXPECTED_MIN_COUNT=1 \
mise run integration:verify
```

For production dump rehearsals, start support services, restore your dump into
the source PostgreSQL URL you choose, then run the migration tasks.

```bash
mise run integration:env:up

POSTGRES_URL=postgres://misskey:misskey@localhost:35432/misskey?sslmode=disable \
AUTH_SECRET=... \
EXCLUDE_TYPES=pollVote \
mise run migration:dry-run

POSTGRES_URL=postgres://misskey:misskey@localhost:35432/misskey?sslmode=disable \
AUTH_SECRET=... \
EXCLUDE_TYPES=pollVote \
mise run migration:import
```
