package services

import (
	"core/app/constants"
	"time"
)

// CRM calendars use the application's China business timezone, independent of host TZ.
func followupDay(now time.Time) (time.Time, time.Time) {
	local := now.In(constants.PRCLocation)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, constants.PRCLocation)
	return start, start.AddDate(0, 0, 1)
}
