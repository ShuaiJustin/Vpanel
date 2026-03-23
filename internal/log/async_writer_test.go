package log

import (
	"context"
	"errors"
	"testing"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

type asyncWriterRepoStub struct {
	createBatchCalls int
	failCreateBatch  int
	stored           []*repository.Log
}

func (r *asyncWriterRepoStub) Create(ctx context.Context, log *repository.Log) error {
	return nil
}

func (r *asyncWriterRepoStub) CreateBatch(ctx context.Context, logs []*repository.Log) error {
	r.createBatchCalls++
	if r.failCreateBatch > 0 {
		r.failCreateBatch--
		return errors.New("create batch failed")
	}

	r.stored = append(r.stored, append([]*repository.Log(nil), logs...)...)
	return nil
}

func (r *asyncWriterRepoStub) GetByID(ctx context.Context, id int64) (*repository.Log, error) {
	return nil, errors.New("not implemented")
}

func (r *asyncWriterRepoStub) List(ctx context.Context, filter *repository.LogFilter, limit, offset int) ([]*repository.Log, error) {
	return nil, errors.New("not implemented")
}

func (r *asyncWriterRepoStub) Count(ctx context.Context, filter *repository.LogFilter) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *asyncWriterRepoStub) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *asyncWriterRepoStub) DeleteByFilter(ctx context.Context, filter *repository.LogFilter) (int64, error) {
	return 0, errors.New("not implemented")
}

func TestAsyncWriterFlushesWhenBatchSizeReached(t *testing.T) {
	repo := &asyncWriterRepoStub{}
	log := logger.New(logger.Config{Level: "debug", Format: "json", Output: "stdout"})
	writer := NewAsyncWriter(repo, log, 10, 2, time.Hour)
	defer writer.Close()

	if err := writer.Write(&repository.Log{Message: "first"}); err != nil {
		t.Fatalf("write first log: %v", err)
	}
	if repo.createBatchCalls != 0 {
		t.Fatalf("expected no flush before batch size is reached, got %d", repo.createBatchCalls)
	}

	if err := writer.Write(&repository.Log{Message: "second"}); err != nil {
		t.Fatalf("write second log: %v", err)
	}

	if repo.createBatchCalls != 1 {
		t.Fatalf("expected one flush after batch size is reached, got %d", repo.createBatchCalls)
	}
	if len(repo.stored) != 2 {
		t.Fatalf("expected 2 stored logs, got %d", len(repo.stored))
	}
	if len(writer.buffer) != 0 {
		t.Fatalf("expected empty buffer after successful flush, got %d", len(writer.buffer))
	}
}

func TestAsyncWriterRetainsBufferAfterFlushFailure(t *testing.T) {
	repo := &asyncWriterRepoStub{failCreateBatch: 1}
	log := logger.New(logger.Config{Level: "debug", Format: "json", Output: "stdout"})
	writer := NewAsyncWriter(repo, log, 10, 2, time.Hour)
	defer writer.Close()

	if err := writer.Write(&repository.Log{Message: "first"}); err != nil {
		t.Fatalf("write first log: %v", err)
	}

	if err := writer.Write(&repository.Log{Message: "second"}); err == nil {
		t.Fatal("expected flush error on second write")
	}

	if len(writer.buffer) != 2 {
		t.Fatalf("expected buffer to retain failed batch, got %d entries", len(writer.buffer))
	}
	if len(repo.stored) != 0 {
		t.Fatalf("expected no stored logs after failed flush, got %d", len(repo.stored))
	}

	writer.flush()

	if len(repo.stored) != 2 {
		t.Fatalf("expected failed batch to be retried and stored, got %d", len(repo.stored))
	}
	if len(writer.buffer) != 0 {
		t.Fatalf("expected empty buffer after retry succeeds, got %d", len(writer.buffer))
	}
}
