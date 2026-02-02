package budgets

import "time"

func PeriodBounds(now time.Time, p Period) (start, end, time.Time) {
	loc := now.Location()
	switch p {
	case PeriodMonth:
		y, m, _ := now.Date()
		start = time.Date(y, m, 1, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 1, 0)

	case PeriodWeek:
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(wd - 1))
		end = start.AddDate(0, 0, 7)
	default:
		y, m, _ := now.Date()
		start = time.Date(y, m, 1, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 1, 0)
	}
	return
}
