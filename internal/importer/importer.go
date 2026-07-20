package importer

import (
	"context"
	"errors"
	"fmt"

	"github.com/mi-24v/miwkey-notification-importer/internal/notification"
	"github.com/mi-24v/miwkey-notification-importer/internal/postgres"
)

const defaultBatchSize = 500

type Reader interface {
	Read(ctx context.Context, opts postgres.ReadOptions) ([]notification.Row, error)
}

type ExtensionClient interface {
	CreateNotification(ctx context.Context, payload notification.Payload) error
}

type Importer struct {
	Reader Reader
	Client ExtensionClient
}

type Options struct {
	DryRun        bool
	Limit         int
	BatchSize     int
	ResumeAfterID string
	ExcludeTypes  []string
}

type Result struct {
	SuccessCount     int
	FailureCount     int
	SkippedCount     int
	LastSuccessfulID string
	FailedID         string
	LastSkippedID    string
}

func (i Importer) Run(ctx context.Context, opts Options) (Result, error) {
	if i.Reader == nil {
		return Result{}, errors.New("reader is required")
	}
	if !opts.DryRun && i.Client == nil {
		return Result{}, errors.New("extension client is required")
	}

	result := Result{}
	resumeAfterID := opts.ResumeAfterID
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	excludeTypes := excludeTypeSet(opts.ExcludeTypes)

	for {
		readBatchSize := batchSize
		if opts.Limit > 0 {
			remaining := opts.Limit - result.SuccessCount
			if remaining <= 0 {
				return result, nil
			}
			if remaining < readBatchSize {
				readBatchSize = remaining
			}
		}

		rows, err := i.Reader.Read(ctx, postgres.ReadOptions{
			BatchSize:     readBatchSize,
			ResumeAfterID: resumeAfterID,
		})
		if err != nil {
			return result, fmt.Errorf("read notifications: %w", err)
		}
		if len(rows) == 0 {
			return result, nil
		}

		for _, row := range rows {
			if _, ok := excludeTypes[row.Type]; ok {
				result.SkippedCount++
				result.LastSkippedID = row.ID
				resumeAfterID = row.ID
				continue
			}

			payload, err := row.ToPayload()
			if err != nil {
				result.FailureCount++
				result.FailedID = row.ID
				return result, fmt.Errorf("notification %s: %w", row.ID, err)
			}

			if !opts.DryRun {
				if err := i.Client.CreateNotification(ctx, payload); err != nil {
					result.FailureCount++
					result.FailedID = row.ID
					return result, fmt.Errorf("notification %s: %w", row.ID, err)
				}
			}

			result.SuccessCount++
			result.LastSuccessfulID = row.ID
			resumeAfterID = row.ID

			if opts.Limit > 0 && result.SuccessCount >= opts.Limit {
				return result, nil
			}
		}

		if len(rows) < readBatchSize {
			return result, nil
		}
	}
}

func excludeTypeSet(types []string) map[string]struct{} {
	if len(types) == 0 {
		return nil
	}

	result := make(map[string]struct{}, len(types))
	for _, notificationType := range types {
		result[notificationType] = struct{}{}
	}
	return result
}
