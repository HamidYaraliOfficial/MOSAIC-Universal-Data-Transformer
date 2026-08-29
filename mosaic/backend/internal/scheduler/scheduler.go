// Package scheduler implements automated pipeline scheduling: the user
// defines the "operating hours" during which a pipeline is allowed to run
// unattended (e.g. only on weekdays, 09:00-18:00 local time), and MOSAIC
// works out — live — whether it's currently within that window, how long
// until the next window opens, and how long the next run is expected to
// take, based on the pipeline's own execution history.
package scheduler

import (
	"fmt"
	"sort"
	"time"
)

// Window is one open interval within a single day, e.g. 09:00-18:00.
// Minutes are minutes-since-midnight in the schedule's configured timezone.
type Window struct {
	StartMinute int `json:"startMinute"`
	EndMinute   int `json:"endMinute"`
}

// Schedule is a user-configured operating-hours definition for one
// pipeline, entered entirely by the user from the Scheduler panel: which
// weekdays run automatically, and the open window(s) for each.
type Schedule struct {
	PipelineID string           `json:"pipelineId"`
	Timezone   string           `json:"timezone"` // IANA name, e.g. "Asia/Tehran"
	Days       map[string]Window `json:"days"`     // "monday".."sunday" -> Window; a day absent from the map is always closed
	Enabled    bool             `json:"enabled"`
}

// RunRecord is one historical execution, used only to estimate how long the
// *next* run will take — MOSAIC never guesses a duration out of thin air.
type RunRecord struct {
	StartedAt time.Time     `json:"startedAt"`
	Duration  time.Duration `json:"duration"`
	RowCount  int           `json:"rowCount"`
}

// Status is the live answer the Scheduler panel renders: is the pipeline's
// window open right now, and if not, exactly how long until it opens next.
type Status struct {
	IsOpenNow        bool          `json:"isOpenNow"`
	Now              time.Time     `json:"now"`
	CurrentWindowEnd *time.Time    `json:"currentWindowEnd,omitempty"`
	NextOpenAt       *time.Time    `json:"nextOpenAt,omitempty"`
	TimeUntilNext    time.Duration `json:"timeUntilNext"`
	EstimatedRunTime time.Duration `json:"estimatedRunTime"`
	SampleSize       int           `json:"sampleSize"`
}

var weekdayNames = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

// Evaluate computes live Status for a Schedule at time `now`, searching up
// to 14 days ahead for the next open window if the schedule is currently
// closed. All inputs (days, windows, timezone) come from what the user
// entered in the Scheduler panel; nothing here is hardcoded.
func Evaluate(s Schedule, history []RunRecord, now time.Time) (Status, error) {
	loc := time.UTC
	if s.Timezone != "" {
		l, err := time.LoadLocation(s.Timezone)
		if err != nil {
			return Status{}, fmt.Errorf("scheduler: invalid timezone %q: %w", s.Timezone, err)
		}
		loc = l
	}
	local := now.In(loc)
	status := Status{Now: now, EstimatedRunTime: estimateDuration(history), SampleSize: len(history)}

	if w, ok := s.Days[weekdayNames[int(local.Weekday())]]; ok {
		minuteOfDay := local.Hour()*60 + local.Minute()
		if minuteOfDay >= w.StartMinute && minuteOfDay < w.EndMinute {
			end := dayStart(local).Add(time.Duration(w.EndMinute) * time.Minute)
			status.IsOpenNow = true
			status.CurrentWindowEnd = &end
			return status, nil
		}
	}

	// Search forward day by day for the next configured window.
	for offset := 0; offset <= 14; offset++ {
		day := local.AddDate(0, 0, offset)
		w, ok := s.Days[weekdayNames[int(day.Weekday())]]
		if !ok {
			continue
		}
		candidate := dayStart(day).Add(time.Duration(w.StartMinute) * time.Minute)
		if candidate.Before(local) {
			continue // today's window already passed
		}
		status.NextOpenAt = &candidate
		status.TimeUntilNext = candidate.Sub(local)
		return status, nil
	}
	return status, nil
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// estimateDuration averages the most recent runs (up to 10) to project how
// long the next run will take — shown next to the countdown so the user
// knows not just *when* the pipeline runs next, but roughly how long it
// will occupy the machine for.
func estimateDuration(history []RunRecord) time.Duration {
	if len(history) == 0 {
		return 0
	}
	sorted := append([]RunRecord(nil), history...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartedAt.After(sorted[j].StartedAt) })
	if len(sorted) > 10 {
		sorted = sorted[:10]
	}
	var total time.Duration
	for _, r := range sorted {
		total += r.Duration
	}
	return total / time.Duration(len(sorted))
}
