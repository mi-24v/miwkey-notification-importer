package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mi-24v/miwkey-notification-importer/internal/auth"
	"github.com/mi-24v/miwkey-notification-importer/internal/extension"
	"github.com/mi-24v/miwkey-notification-importer/internal/importer"
	"github.com/mi-24v/miwkey-notification-importer/internal/postgres"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	var cfg config
	flags := flag.NewFlagSet("miwkey-notification-importer", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.postgresURL, "postgres-url", "", "restored PostgreSQL connection URL")
	flags.StringVar(&cfg.extensionURL, "extension-url", "", "notification extension server base URL")
	flags.StringVar(&cfg.bearerToken, "bearer-token", "", "pre-generated bearer token")
	flags.StringVar(&cfg.secret, "secret", "", "shared secret for HS256 JWT generation")
	flags.BoolVar(&cfg.dryRun, "dry-run", false, "validate and count rows without posting to the extension server")
	flags.IntVar(&cfg.limit, "limit", 0, "maximum number of notifications to import")
	flags.StringVar(&cfg.resumeAfterID, "resume-after-id", "", "resume after this notification id")
	flags.IntVar(&cfg.batchSize, "batch-size", 500, "number of rows to read per PostgreSQL query")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := cfg.validate(); err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.postgresURL)
	if err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}
	defer pool.Close()

	imp := importer.Importer{
		Reader: postgres.Reader{DB: poolDB{pool: pool}},
		Client: extension.Client{
			BaseURL: cfg.extensionURL,
			HTTPClient: &http.Client{
				Timeout: 30 * time.Second,
			},
			TokenSource: cfg.tokenSource(),
		},
	}

	result, err := imp.Run(ctx, importer.Options{
		DryRun:        cfg.dryRun,
		Limit:         cfg.limit,
		BatchSize:     cfg.batchSize,
		ResumeAfterID: cfg.resumeAfterID,
	})
	printResult(result)
	if err != nil {
		return err
	}
	return nil
}

type config struct {
	postgresURL   string
	extensionURL  string
	bearerToken   string
	secret        string
	dryRun        bool
	limit         int
	resumeAfterID string
	batchSize     int
}

func (c config) validate() error {
	if c.postgresURL == "" {
		return fmt.Errorf("--postgres-url is required")
	}
	if c.extensionURL == "" {
		return fmt.Errorf("--extension-url is required")
	}
	if (c.bearerToken == "") == (c.secret == "") {
		return fmt.Errorf("exactly one of --bearer-token or --secret is required")
	}
	if c.limit < 0 {
		return fmt.Errorf("--limit must be >= 0")
	}
	if c.batchSize <= 0 {
		return fmt.Errorf("--batch-size must be > 0")
	}
	return nil
}

func (c config) tokenSource() auth.TokenSource {
	if c.bearerToken != "" {
		return auth.StaticBearer(c.bearerToken)
	}
	return auth.JWTSource{Secret: c.secret}
}

func printResult(result importer.Result) {
	fmt.Fprintf(os.Stderr, "success=%d failure=%d last_successful_id=%q failed_id=%q\n",
		result.SuccessCount,
		result.FailureCount,
		result.LastSuccessfulID,
		result.FailedID,
	)
}

type poolDB struct {
	pool *pgxpool.Pool
}

func (db poolDB) Query(ctx context.Context, sql string, args ...any) (postgres.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

func (db poolDB) QueryRow(ctx context.Context, sql string, args ...any) postgres.RowScanner {
	return db.pool.QueryRow(ctx, sql, args...)
}
