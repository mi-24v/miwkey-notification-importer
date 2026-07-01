package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mi-24v/miwkey-notification-importer/internal/postgres"
)

func TestBuildListQueryWithoutResume(t *testing.T) {
	query, args := postgres.BuildListQuery(100, nil)

	if strings.Contains(query, "WHERE") {
		t.Fatalf("query contains WHERE: %s", query)
	}
	if !strings.Contains(query, `ORDER BY "createdAt", id`) {
		t.Fatalf("query missing stable order: %s", query)
	}
	if !strings.Contains(query, "LIMIT $1") {
		t.Fatalf("query missing limit placeholder: %s", query)
	}
	if len(args) != 1 || args[0] != 100 {
		t.Fatalf("args = %#v", args)
	}
}

func TestBuildListQueryWithResume(t *testing.T) {
	createdAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	query, args := postgres.BuildListQuery(50, &postgres.ResumeBoundary{
		CreatedAt: createdAt,
		ID:        "resume-id",
	})

	if !strings.Contains(query, `WHERE ("createdAt", id) > ($1, $2)`) {
		t.Fatalf("query missing resume boundary: %s", query)
	}
	if !strings.Contains(query, "LIMIT $3") {
		t.Fatalf("query missing limit placeholder: %s", query)
	}
	if len(args) != 3 || args[0] != createdAt || args[1] != "resume-id" || args[2] != 50 {
		t.Fatalf("args = %#v", args)
	}
}

func TestReaderReadMapsNullableColumns(t *testing.T) {
	createdAt := time.Date(2022, 3, 4, 5, 6, 7, 0, time.UTC)
	db := &fakeDB{
		rows: &fakeRows{
			values: [][]any{{
				"notification-id",
				createdAt,
				"notifiee",
				pgtype.Text{},
				"follow",
				true,
				pgtype.Text{},
				pgtype.Text{String: ":smile:", Valid: true},
				pgtype.Int4{},
				pgtype.Text{},
				pgtype.Text{},
				pgtype.Text{},
				pgtype.Text{},
				pgtype.Text{},
			}},
		},
	}
	reader := postgres.Reader{DB: db}

	rows, err := reader.Read(context.Background(), postgres.ReadOptions{BatchSize: 10})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d", len(rows))
	}

	row := rows[0]
	if row.ID != "notification-id" {
		t.Fatalf("ID = %q", row.ID)
	}
	if row.NotifierID != nil {
		t.Fatalf("NotifierID = %v", row.NotifierID)
	}
	if row.NoteID != nil {
		t.Fatalf("NoteID = %v", row.NoteID)
	}
	if row.Reaction == nil || *row.Reaction != ":smile:" {
		t.Fatalf("Reaction = %v", row.Reaction)
	}
	if row.Choice != nil {
		t.Fatalf("Choice = %v", row.Choice)
	}
}

type fakeDB struct {
	rows *fakeRows
}

func (db *fakeDB) Query(_ context.Context, _ string, _ ...any) (postgres.Rows, error) {
	return db.rows, nil
}

func (db *fakeDB) QueryRow(_ context.Context, _ string, _ ...any) postgres.RowScanner {
	return fakeRow{err: errors.New("not used")}
}

type fakeRow struct {
	err error
}

func (r fakeRow) Scan(_ ...any) error {
	return r.err
}

type fakeRows struct {
	values [][]any
	index  int
	err    error
}

func (r *fakeRows) Close() {}

func (r *fakeRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index >= len(r.values) {
		return errors.New("scan past end")
	}
	for i, value := range r.values[r.index] {
		switch d := dest[i].(type) {
		case *string:
			*d = value.(string)
		case *time.Time:
			*d = value.(time.Time)
		case *bool:
			*d = value.(bool)
		case *pgtype.Text:
			*d = value.(pgtype.Text)
		case *pgtype.Int4:
			*d = value.(pgtype.Int4)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	r.index++
	return nil
}

func (r *fakeRows) Err() error {
	return r.err
}
