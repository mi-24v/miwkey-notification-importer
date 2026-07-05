package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mi-24v/miwkey-notification-importer/internal/notification"
)

const defaultBatchSize = 500

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) RowScanner
}

type Rows interface {
	Close()
	Next() bool
	Scan(dest ...any) error
	Err() error
}

type RowScanner interface {
	Scan(dest ...any) error
}

type Reader struct {
	DB DB
}

type ReadOptions struct {
	BatchSize     int
	ResumeAfterID string
}

type ResumeBoundary struct {
	CreatedAt time.Time
	ID        string
}

type NotificationColumns struct {
	Achievement bool
}

func (r Reader) Read(ctx context.Context, opts ReadOptions) ([]notification.Row, error) {
	var resume *ResumeBoundary
	if opts.ResumeAfterID != "" {
		boundary, err := r.findResumeBoundary(ctx, opts.ResumeAfterID)
		if err != nil {
			return nil, err
		}
		resume = &boundary
	}

	columns, err := r.readNotificationColumns(ctx)
	if err != nil {
		return nil, err
	}

	query, args := BuildListQueryForColumns(opts.BatchSize, resume, columns)
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	var notifications []notification.Row
	for rows.Next() {
		row, err := scanNotificationRow(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read notifications: %w", err)
	}

	return notifications, nil
}

func (r Reader) readNotificationColumns(ctx context.Context) (NotificationColumns, error) {
	rows, err := r.DB.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'notification' AND column_name IN ('achievement')`)
	if err != nil {
		return NotificationColumns{}, fmt.Errorf("inspect notification columns: %w", err)
	}
	defer rows.Close()

	var columns NotificationColumns
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return NotificationColumns{}, fmt.Errorf("scan notification column: %w", err)
		}
		if columnName == "achievement" {
			columns.Achievement = true
		}
	}
	if err := rows.Err(); err != nil {
		return NotificationColumns{}, fmt.Errorf("read notification columns: %w", err)
	}

	return columns, nil
}

func (r Reader) findResumeBoundary(ctx context.Context, id string) (ResumeBoundary, error) {
	var boundary ResumeBoundary
	err := r.DB.QueryRow(ctx, `SELECT "createdAt", id FROM notification WHERE id = $1`, id).Scan(
		&boundary.CreatedAt,
		&boundary.ID,
	)
	if err != nil {
		return ResumeBoundary{}, fmt.Errorf("find resume boundary %s: %w", id, err)
	}
	return boundary, nil
}

func BuildListQuery(limit int, resume *ResumeBoundary) (string, []any) {
	return BuildListQueryForColumns(limit, resume, NotificationColumns{Achievement: true})
}

func BuildListQueryForColumns(limit int, resume *ResumeBoundary, columns NotificationColumns) (string, []any) {
	if limit <= 0 {
		limit = defaultBatchSize
	}

	var builder strings.Builder
	builder.WriteString(`SELECT id, "createdAt", "notifieeId", "notifierId", type, "isRead", "noteId", reaction, choice, "customBody", "customHeader", "customIcon", "appAccessTokenId", `)
	if columns.Achievement {
		builder.WriteString(`achievement`)
	} else {
		builder.WriteString(`NULL::varchar AS achievement`)
	}
	builder.WriteString(` FROM notification`)

	if resume != nil {
		builder.WriteString(` WHERE ("createdAt", id) > ($1, $2)`)
		builder.WriteString(` ORDER BY "createdAt", id LIMIT $3`)
		return builder.String(), []any{resume.CreatedAt, resume.ID, limit}
	}

	builder.WriteString(` ORDER BY "createdAt", id LIMIT $1`)
	return builder.String(), []any{limit}
}

func scanNotificationRow(rows Rows) (notification.Row, error) {
	var row notification.Row
	var notifierID pgtype.Text
	var noteID pgtype.Text
	var reaction pgtype.Text
	var choice pgtype.Int4
	var customBody pgtype.Text
	var customHeader pgtype.Text
	var customIcon pgtype.Text
	var appAccessTokenID pgtype.Text
	var achievement pgtype.Text

	err := rows.Scan(
		&row.ID,
		&row.CreatedAt,
		&row.NotifieeID,
		&notifierID,
		&row.Type,
		&row.IsRead,
		&noteID,
		&reaction,
		&choice,
		&customBody,
		&customHeader,
		&customIcon,
		&appAccessTokenID,
		&achievement,
	)
	if err != nil {
		return notification.Row{}, fmt.Errorf("scan notification row: %w", err)
	}

	row.NotifierID = textPtr(notifierID)
	row.NoteID = textPtr(noteID)
	row.Reaction = textPtr(reaction)
	row.Choice = intPtr(choice)
	row.CustomBody = textPtr(customBody)
	row.CustomHeader = textPtr(customHeader)
	row.CustomIcon = textPtr(customIcon)
	row.AppAccessTokenID = textPtr(appAccessTokenID)
	row.Achievement = textPtr(achievement)

	return row, nil
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func intPtr(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int32)
	return &v
}
