# Miwkey Notification Importer Design

## Goal

Misskey 12.119.0 以前の `notification` table を、復元済み PostgreSQL から読み取り、miwkey-extension の notification extension server へ import する一回限りの移行用 CLI を作る。

移行元 dump は既存手順の `pg_dump -f dump.sql` をそのまま取得し、その dump を一時 PostgreSQL コンテナへ restore してから importer が直読みする。importer は raw `dump.sql` を解析しない。

## Scope

対象に含めるもの:

- PostgreSQL の `notification` table からの順次読み取り
- `createdAt, id` 順での安定した import
- extension server の `POST /api/v1/notifications` への送信
- Misskey 旧 notification table の主要カラムから extension server の JSON payload への変換
- Bearer token または共有 secret から生成する短命 JWT での認証
- `--dry-run`, `--limit`, `--resume-after-id`, `--fail-fast` による移行作業支援
- 成功件数、失敗件数、最後に処理した notification id のログ出力

対象に含めないもの:

- raw `pg_dump` SQL の parser
- dump format の複数対応
- PostgreSQL 以外の移行元
- extension server からの読み戻し検証
- Misskey 本体との同期処理
- grouped notification の生成

## Inputs

CLI は復元済み PostgreSQL へ接続する。

必須オプション:

- `--postgres-url`: 復元済み PostgreSQL の connection URL
- `--extension-url`: notification extension server の base URL

認証オプションはいずれか一つを必須にする。

- `--bearer-token`: 既に生成済みの Bearer token をそのまま使う
- `--secret`: Misskey fork と extension server が共有する secret から HS256 JWT を生成する

補助オプション:

- `--dry-run`: DB 読み取りと payload 生成だけを行い、HTTP POST しない
- `--limit`: 最大 import 件数
- `--resume-after-id`: 指定 id の次の行から再開する
- `--fail-fast`: 最初の失敗で停止する
- `--batch-size`: DB から一度に読む件数。HTTP は一件ずつ送る

## PostgreSQL Query

importer は `notification` table を主キーと作成日時で安定順序に並べて読む。

基本 query:

```sql
SELECT
  id,
  "createdAt",
  "notifieeId",
  "notifierId",
  type,
  "isRead",
  "noteId",
  reaction,
  choice,
  "customBody",
  "customHeader",
  "customIcon",
  "appAccessTokenId",
  achievement
FROM notification
ORDER BY "createdAt", id;
```

`--resume-after-id` 指定時は、最初に対象 id の `createdAt` を取得し、次の条件で再開する。

```sql
WHERE ("createdAt", id) > ($1, $2)
ORDER BY "createdAt", id
```

これにより、ID アルゴリズムに依存せず時刻順で再開できる。

## Payload Mapping

extension server は `POST /api/v1/notifications` で `BaseNotification` 互換 payload を受け取る。importer は DB の NULL を JSON の omit または null に変換し、空文字列を勝手に補完しない。

共通 mapping:

- `id` -> `id`
- `createdAt` -> `createdAt`
- `notifieeId` -> `notifieeId`
- `notifierId` -> `notifierId`。NULL の場合は省略
- `type` -> `type`
- `isRead` -> `isRead`
- `noteId` -> `noteId`。NULL の場合は省略
- `reaction` -> `reaction`。NULL の場合は省略
- `customBody` -> `customBody`。NULL の場合は省略
- `customHeader` -> `customHeader`。NULL の場合は省略
- `customIcon` -> `customIcon`。NULL の場合は省略
- `appAccessTokenId` -> `appAccessTokenId`。NULL の場合は省略
- `achievement` -> `achievement`。NULL の場合は省略

`choice` は現行 extension server model に対応フィールドがないため送信しない。旧 `pollVote` が残っている場合は、extension server が未知 type を格納できない可能性があるため、importer はエラーとして扱い、`--fail-fast=false` のときは失敗件数に記録して続行する。

## HTTP Behavior

importer は各 notification を以下へ送信する。

```text
POST {extension-url}/api/v1/notifications
Authorization: Bearer <token>
Content-Type: application/json
```

成功扱い:

- HTTP 200
- HTTP 201
- HTTP 204

失敗扱い:

- 4xx
- 5xx
- timeout
- response body が読めない通信エラー

失敗時は notification id、HTTP status、短い error message をログへ出す。`--fail-fast` が true の場合は即終了する。false の場合は続行し、最後に非ゼロ exit code で終了する。

## Authentication

`--bearer-token` が指定された場合、token をそのまま Authorization header に使う。

`--secret` が指定された場合、Misskey fork 側と同じ形の HS256 JWT を生成する。

claims:

- `iss`: `miwkey-notification-importer`
- `sub`: `notification-extension`
- `iat`: 現在時刻
- `exp`: 現在時刻から 60 秒後

長い import 中でも token 期限切れを避けるため、HTTP request ごとに JWT を生成する。

## Error Handling

DB 接続失敗、query 失敗、必須カラム欠落は即終了する。

行単位の validation 失敗や HTTP 失敗は `--fail-fast` に従う。失敗を継続する場合も、最後に成功件数、失敗件数、最後に成功した id、最後に処理した id を表示する。

`--dry-run` では HTTP 送信を行わず、payload validation と件数集計だけを実行する。

## Testing

TDD で以下を確認する。

- DB row から JSON payload へ正しく変換できる
- nullable fields が空文字列に変わらず、省略または null として扱われる
- `resume-after-id` が `createdAt, id` 順で次の行から再開する
- HTTP client が Authorization header と JSON body を送る
- 2xx を成功、4xx/5xx を失敗として扱う
- `--fail-fast` の有無で停止条件が変わる
- `--dry-run` では HTTP request を送らない

## Repository Shape

新 repo は Go CLI として構成する。

予定ファイル:

- `go.mod`: module と依存関係
- `cmd/miwkey-notification-importer/main.go`: CLI entrypoint
- `internal/importer`: import orchestration
- `internal/postgres`: notification row reader
- `internal/extension`: extension server HTTP client
- `internal/auth`: Bearer token と JWT 生成
- `internal/notification`: row to payload mapping
- `README.md`: dump restore から importer 実行までの手順

## Migration Procedure

想定手順:

```bash
pg_dump -h example.rds.amazonaws.com -U postgres -d misskey -f dump.sql
```

dump を一時 PostgreSQL コンテナへ restore する。

```bash
psql "$TEMP_POSTGRES_URL" -f dump.sql
```

importer を dry-run する。

```bash
miwkey-notification-importer \
  --postgres-url "$TEMP_POSTGRES_URL" \
  --extension-url "$EXTENSION_URL" \
  --secret "$NOTIFICATION_EXTENSION_SECRET" \
  --dry-run
```

問題がなければ import する。

```bash
miwkey-notification-importer \
  --postgres-url "$TEMP_POSTGRES_URL" \
  --extension-url "$EXTENSION_URL" \
  --secret "$NOTIFICATION_EXTENSION_SECRET"
```

失敗後は最後に処理した id を使って再開する。

```bash
miwkey-notification-importer \
  --postgres-url "$TEMP_POSTGRES_URL" \
  --extension-url "$EXTENSION_URL" \
  --secret "$NOTIFICATION_EXTENSION_SECRET" \
  --resume-after-id "$LAST_PROCESSED_ID"
```
