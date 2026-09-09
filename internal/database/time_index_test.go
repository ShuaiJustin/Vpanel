package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLiteTimeRangeUsesExpressionIndex(t *testing.T) {
	db, err := New(&Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "index.db")})
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.AutoMigrate())
	require.NoError(t, db.ensurePerformanceIndexes(context.Background()))
	var rows []struct{ Detail string }
	require.NoError(t, db.db.Raw("EXPLAIN QUERY PLAN SELECT node_id,SUM(upload),SUM(download) FROM node_traffic WHERE datetime(recorded_at) BETWEEN datetime(?) AND datetime(?) GROUP BY node_id", "2026-09-09T10:00:00+08:00", "2026-09-09T10:05:00+08:00").Scan(&rows).Error)
	found := false
	for _, row := range rows {
		if strings.Contains(row.Detail, "SEARCH node_traffic USING INDEX idx_node_traffic_utc_range") {
			found = true
		}
	}
	require.True(t, found, "query must range-search rather than scan: %+v", rows)
	// Equivalent instants with different timezone strings must still match.
	require.NoError(t, db.db.Exec("INSERT INTO node_traffic(node_id,user_id,upload,download,recorded_at) VALUES (1,1,5,7,?), (1,1,3,2,?)", "2026-09-09 10:01:00+08:00", "2026-09-09 02:02:00+00:00").Error)
	var count int64
	require.NoError(t, db.db.Raw("SELECT COUNT(*) FROM node_traffic WHERE datetime(recorded_at) BETWEEN datetime(?) AND datetime(?)", "2026-09-09T10:00:00+08:00", "2026-09-09T10:05:00+08:00").Scan(&count).Error)
	require.Equal(t, int64(2), count)
}
