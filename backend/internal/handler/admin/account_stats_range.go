package admin

import (
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// Explicit dates are inclusive calendar days in the user's timezone.
func accountStatsTimeRange(days int, startDate, endDate, userTZ string) (time.Time, time.Time, error) {
	now := timezone.NowInUserLocation(userTZ)
	if startDate == "" && endDate == "" {
		return timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -days+1), userTZ),
			timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ), nil
	}
	start, err := timezone.ParseInUserLocation(time.DateOnly, startDate, userTZ)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date and end_date must be valid YYYY-MM-DD dates")
	}
	end, err := timezone.ParseInUserLocation(time.DateOnly, endDate, userTZ)
	if err != nil || end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end_date must be a valid YYYY-MM-DD date on or after start_date")
	}
	end = end.AddDate(0, 0, 1)
	if end.After(start.AddDate(0, 0, 90)) {
		return time.Time{}, time.Time{}, fmt.Errorf("date range must not exceed 90 days")
	}
	return start, end, nil
}
