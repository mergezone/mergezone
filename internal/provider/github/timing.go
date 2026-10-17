package github

import (
	"sort"
	"sync"
	"time"

	"merge/internal/provider"
)

type gatherInterval struct {
	start time.Time
	end   time.Time
}

type gatherWaitTracker struct {
	mu        sync.Mutex
	intervals []gatherInterval
}

func (tracker *gatherWaitTracker) add(start, end time.Time) {
	tracker.mu.Lock()
	tracker.intervals = append(tracker.intervals, gatherInterval{start: start, end: end})
	tracker.mu.Unlock()
}

// duration returns the wall-clock union of GitHub waits, so overlapping scrape
// requests are not counted multiple times.
func (tracker *gatherWaitTracker) duration() time.Duration {
	tracker.mu.Lock()
	intervals := append([]gatherInterval(nil), tracker.intervals...)
	tracker.mu.Unlock()
	if len(intervals) == 0 {
		return 0
	}

	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].start.Before(intervals[j].start)
	})

	start, end := intervals[0].start, intervals[0].end
	var total time.Duration
	for _, interval := range intervals[1:] {
		if interval.start.After(end) {
			total += end.Sub(start)
			start, end = interval.start, interval.end
			continue
		}
		if interval.end.After(end) {
			end = interval.end
		}
	}
	return total + end.Sub(start)
}

func pullRequestTimings(_ time.Time, gatherWaits *gatherWaitTracker) provider.PullRequestTimings {
	return provider.PullRequestTimings{Gather: gatherWaits.duration()}
}
