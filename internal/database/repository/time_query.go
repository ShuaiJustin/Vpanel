package repository

import (
	"fmt"
	"strings"
	"time"
)

// BuildTimeRangeCondition returns a dialect-aware time range filter.
func BuildTimeRangeCondition(dialect, column string) string {
	switch dialect {
	case "sqlite":
		return fmt.Sprintf("datetime(%s) BETWEEN datetime(?) AND datetime(?)", column)
	default:
		return fmt.Sprintf("%s BETWEEN ? AND ?", column)
	}
}

// BuildTimeRangeArgs normalizes time range arguments for the active dialect.
func BuildTimeRangeArgs(dialect string, start, end time.Time) []any {
	switch dialect {
	case "sqlite":
		const sqliteTimeLayout = "2006-01-02 15:04:05.999999999-07:00"
		return []any{start.Format(sqliteTimeLayout), end.Format(sqliteTimeLayout)}
	default:
		return []any{start, end}
	}
}

// BuildTimeGroupingClause returns a dialect-aware grouping expression.
func BuildTimeGroupingClause(dialect, column, interval string) string {
	switch dialect {
	case "sqlite":
		localizedColumn := fmt.Sprintf("datetime(%s, 'localtime')", column)
		switch interval {
		case "hour":
			return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:00:00', %s)", localizedColumn)
		case "day":
			return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s)", localizedColumn)
		case "month":
			return fmt.Sprintf("strftime('%%Y-%%m', %s)", localizedColumn)
		default:
			return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:00:00', %s)", localizedColumn)
		}
	case "mysql":
		switch interval {
		case "hour":
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:00:00')", column)
		case "day":
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d')", column)
		case "month":
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m')", column)
		default:
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:00:00')", column)
		}
	default:
		switch interval {
		case "hour":
			return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM-DD HH24:00:00')", column)
		case "day":
			return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM-DD')", column)
		case "month":
			return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM')", column)
		default:
			return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM-DD HH24:00:00')", column)
		}
	}
}

// BuildTimeMaxExpr returns a dialect-aware max timestamp expression.
func BuildTimeMaxExpr(dialect, column string) string {
	switch dialect {
	case "sqlite":
		return fmt.Sprintf("MAX(datetime(%s, 'localtime'))", column)
	default:
		return fmt.Sprintf("MAX(%s)", column)
	}
}

// ParseAggregatedTime parses timestamp strings returned by aggregate queries.
func ParseAggregatedTime(dialect, raw string, loc *time.Location) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	if loc == nil {
		loc = time.Local
	}

	layoutsWithLocation := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02",
	}
	layoutsWithoutLocation := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
	}

	if dialect == "sqlite" {
		for _, layout := range layoutsWithLocation {
			if parsed, err := time.ParseInLocation(layout, raw, loc); err == nil {
				return &parsed, nil
			}
		}
	}

	for _, layout := range layoutsWithoutLocation {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, nil
		}
	}

	for _, layout := range layoutsWithLocation {
		if parsed, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("unsupported aggregated time value %q", raw)
}
