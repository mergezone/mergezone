package github

import (
	"testing"
	"time"
)

func TestGatherWaitTrackerMergesOverlappingRequests(t *testing.T) {
	base := time.Now()
	tracker := &gatherWaitTracker{}
	tracker.add(base, base.Add(100*time.Millisecond))
	tracker.add(base.Add(50*time.Millisecond), base.Add(150*time.Millisecond))
	tracker.add(base.Add(200*time.Millisecond), base.Add(250*time.Millisecond))

	if got, want := tracker.duration(), 200*time.Millisecond; got != want {
		t.Fatalf("gather wait duration = %s, want %s", got, want)
	}
}
