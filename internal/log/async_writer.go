package log

import (
	"context"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

// AsyncWriter provides asynchronous batch writing of log entries.
type AsyncWriter struct {
	repo          repository.LogRepository
	logger        logger.Logger
	buffer        []*repository.Log
	bufferSize    int
	batchSize     int
	flushInterval time.Duration
	mu            sync.Mutex
	stopCh        chan struct{}
	doneCh        chan struct{}
}

// NewAsyncWriter creates a new async writer.
func NewAsyncWriter(repo repository.LogRepository, log logger.Logger, bufferSize, batchSize int, flushInterval time.Duration) *AsyncWriter {
	if batchSize <= 0 {
		batchSize = bufferSize
	}
	if bufferSize > 0 && batchSize > bufferSize {
		batchSize = bufferSize
	}

	w := &AsyncWriter{
		repo:          repo,
		logger:        log,
		buffer:        make([]*repository.Log, 0, bufferSize),
		bufferSize:    bufferSize,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	go w.flushLoop()
	return w
}

// Write adds a log entry to the buffer.
func (w *AsyncWriter) Write(log *repository.Log) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer = append(w.buffer, log)
	if w.bufferSize > 0 && len(w.buffer) > w.bufferSize {
		dropped := len(w.buffer) - w.bufferSize
		w.buffer = append([]*repository.Log(nil), w.buffer[dropped:]...)
		w.logger.Error("log buffer full, dropping oldest buffered entries", logger.F("dropped", dropped))
	}

	if len(w.buffer) >= w.batchSize {
		return w.flushLocked()
	}

	return nil
}

// flushLoop periodically flushes the buffer.
func (w *AsyncWriter) flushLoop() {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.flush()
		case <-w.stopCh:
			w.flush()
			return
		}
	}
}

// flush flushes the buffer to the database.
func (w *AsyncWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.flushLocked()
}

// flushLocked flushes the buffer (must be called with lock held).
func (w *AsyncWriter) flushLocked() error {
	if len(w.buffer) == 0 {
		return nil
	}

	logs := append([]*repository.Log(nil), w.buffer...)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := w.repo.CreateBatch(ctx, logs); err != nil {
		w.logger.Error("failed to write logs to database", logger.F("error", err), logger.F("count", len(logs)))
		return err
	}

	w.buffer = make([]*repository.Log, 0, w.bufferSize)
	return nil
}

// Close gracefully shuts down the writer.
func (w *AsyncWriter) Close() error {
	close(w.stopCh)
	<-w.doneCh
	return nil
}
