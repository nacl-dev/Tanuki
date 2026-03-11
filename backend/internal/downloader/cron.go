package downloader

import (
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func newCronParser() cron.Parser {
	return cron.NewParser(
		cron.Minute |
			cron.Hour |
			cron.Dom |
			cron.Month |
			cron.Dow |
			cron.Descriptor,
	)
}

// ValidateCronExpression normalizes and validates a standard 5-field cron
// expression and returns its next scheduled run time.
func ValidateCronExpression(raw string) (string, time.Time, error) {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	schedule, err := newCronParser().Parse(normalized)
	if err != nil {
		return "", time.Time{}, err
	}
	return normalized, schedule.Next(time.Now()), nil
}
