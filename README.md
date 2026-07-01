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

Exactly one of `--secret` or `--bearer-token` is required.

## Failure Behavior

The importer stops on the first invalid row or HTTP error. It prints:

- success count
- failure count
- last successful notification id
- failed notification id

It never silently skips failed records.
