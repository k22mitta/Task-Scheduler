package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

func NextRun(cronExpr string, from time.Time) (time.Time, error) {
	p := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := p.Parse(cronExpr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
	}
	return schedule.Next(from), nil
}
