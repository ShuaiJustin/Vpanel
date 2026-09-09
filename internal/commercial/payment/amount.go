package payment

import (
	"fmt"
	"strconv"
	"strings"
)

// parseCents never passes monetary values through floating point.
func parseCents(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid payment amount %q", value)
	}
	fraction := "00"
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, fmt.Errorf("invalid payment amount %q", value)
		}
		fraction = parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
	}
	digits := parts[0] + fraction
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("invalid payment amount %q", value)
		}
	}
	return strconv.ParseInt(digits, 10, 64)
}

func formatCents(amount int64) string { return fmt.Sprintf("%d.%02d", amount/100, amount%100) }
