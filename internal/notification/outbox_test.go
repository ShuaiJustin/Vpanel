package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOutboxRetainsFailuresAcrossRestartAndDeduplicatesChannels(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "outbox")
	cfg := &NotificationConfig{AdminEmail: "admin@example.com", TelegramChatID: "123", EnabledChannels: map[NotificationChannel]bool{ChannelEmail: true, ChannelTelegram: true}}
	svc := NewService(cfg)
	require.NoError(t, svc.ConfigureOutbox(dir))
	require.NoError(t, svc.enqueueAdmin("certificate:1:expiry:7", "expiry", "body"))
	require.NoError(t, svc.enqueueAdmin("certificate:1:expiry:7", "expiry", "later sweep"))
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 2)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), info.Mode().Perm())
	for _, f := range files {
		info, err := os.Stat(filepath.Join(dir, f.Name()))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	now := time.Now().Add(time.Second)
	counts := map[NotificationChannel]int{}
	err = svc.processOutbox(context.Background(), now, func(j *deliveryJob) error {
		counts[j.Channel]++
		if j.Channel == ChannelTelegram {
			return fmt.Errorf("temporary failure")
		}
		return nil
	})
	require.Error(t, err)
	require.Equal(t, 1, counts[ChannelEmail])
	require.Equal(t, 1, counts[ChannelTelegram])
	// Reconstruct the service: retry state lives on disk, not in this process.
	restarted := NewService(cfg)
	require.NoError(t, restarted.ConfigureOutbox(dir))
	require.NoError(t, restarted.processOutbox(context.Background(), now.Add(30*time.Second), func(j *deliveryJob) error { t.Fatal("retried before backoff"); return nil }))
	require.NoError(t, restarted.processOutbox(context.Background(), now.Add(2*time.Minute), func(j *deliveryJob) error { counts[j.Channel]++; return nil }))
	require.Equal(t, 1, counts[ChannelEmail])
	require.Equal(t, 2, counts[ChannelTelegram])
	require.NoError(t, restarted.enqueueAdmin("certificate:1:expiry:7", "expiry", "again"))
	require.NoError(t, restarted.processOutbox(context.Background(), now.Add(3*time.Minute), func(j *deliveryJob) error { t.Fatal("delivered duplicate"); return nil }))
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		require.NoError(t, err)
		var j deliveryJob
		require.NoError(t, json.Unmarshal(data, &j))
		require.NotNil(t, j.DeliveredAt)
	}
	require.NoError(t, restarted.processOutbox(context.Background(), now.Add(181*24*time.Hour), func(j *deliveryJob) error { return nil }))
	files, err = os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestOutboxRejectsUnsafeRoot(t *testing.T) {
	svc := NewService(nil)
	require.Error(t, svc.ConfigureOutbox("/"))
	require.Error(t, svc.ConfigureOutbox("relative"))
}

func TestOutboxCancelsJobsWhenChannelDisabled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "outbox")
	svc := NewService(&NotificationConfig{AdminEmail: "admin@example.com", EnabledChannels: map[NotificationChannel]bool{ChannelEmail: true}})
	require.NoError(t, svc.ConfigureOutbox(dir))
	require.NoError(t, svc.enqueueAdmin("test", "subject", "body"))
	svc.UpdateConfig(&NotificationConfig{EnabledChannels: map[NotificationChannel]bool{}})
	require.NoError(t, svc.processOutbox(context.Background(), time.Now().Add(time.Second), svc.deliverJob))
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	data, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	require.NoError(t, err)
	var job deliveryJob
	require.NoError(t, json.Unmarshal(data, &job))
	require.NotNil(t, job.CancelledAt)
	require.Nil(t, job.DeliveredAt)
	require.Zero(t, job.Attempts)
	require.NoError(t, svc.processOutbox(context.Background(), time.Now().Add(time.Hour), func(*deliveryJob) error { t.Fatal("cancelled job retried"); return nil }))
}
