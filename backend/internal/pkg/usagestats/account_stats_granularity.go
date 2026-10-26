package usagestats

import "time"

// AccountStatsBucket identifies an elapsed hour or a local calendar day.
// UTC hour keys keep the repeated hour at the end of DST distinct.
func AccountStatsBucket(at time.Time, granularity string) (key, label string) {
	if granularity == "hour" {
		return at.UTC().Format(time.RFC3339), at.Format("01/02 15:04")
	}
	return at.Format("2006-01-02"), at.Format("01/02")
}

func NextAccountStatsBucket(at time.Time, granularity string) time.Time {
	if granularity == "hour" {
		return at.Add(time.Hour)
	}
	return at.AddDate(0, 0, 1)
}
