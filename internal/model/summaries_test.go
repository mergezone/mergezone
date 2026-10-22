package model

import (
	"slices"
	"testing"
	"time"
)

func summaryTime(day int) *time.Time {
	value := time.Date(2025, time.January, day, 0, 0, 0, 0, time.UTC)
	return &value
}

func scopeOrder(scopes []ScopeInfo) []string {
	names := make([]string, len(scopes))
	for i, scope := range scopes {
		names[i] = scope.Name
	}
	return names
}

func contributorOrder(contributors []ContributorInfo) []string {
	names := make([]string, len(contributors))
	for i, contributor := range contributors {
		names[i] = contributor.Name
	}
	return names
}

// summariesNow is the reference time; creation times are counted back from it.
var summariesNow = time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)

func scopeSummaryPR(scope string, createdDay, updatedDay int) StampedPullRequest {
	createdAt := summaryTime(createdDay)
	updatedAt := summaryTime(updatedDay)
	return StampedPullRequest{
		PullRequest: PullRequest{CreatedAt: createdAt, UpdatedAt: updatedAt},
		TimeOpen:    summariesNow.Sub(*createdAt),
		DaysOpen:    int(summariesNow.Sub(*createdAt).Hours() / 24),
		Scopes:      []string{scope},
	}
}

func TestScopeAgesSortsByNewestCreationTime(t *testing.T) {
	prs := []StampedPullRequest{
		scopeSummaryPR("old", 1, 1),
		scopeSummaryPR("new", 5, 5),
		scopeSummaryPR("middle", 3, 3),
	}

	want := []string{"new", "middle", "old"}
	if got := scopeOrder(ScopeAges(prs)); !slices.Equal(got, want) {
		t.Errorf("ScopeAges() order = %v, want %v", got, want)
	}
}

// A scope whose pull request was updated most recently should still rank by the
// age the card displays (time since the newest pull request was created).
func TestScopeAgesRanksByCreationNotUpdate(t *testing.T) {
	prs := []StampedPullRequest{
		// Created long ago but updated most recently: the old sort key ranked
		// this scope first while displaying an older age than the scope below.
		scopeSummaryPR("alpha", 1, 9),
		scopeSummaryPR("alpha", 3, 3),
		scopeSummaryPR("beta", 5, 5),
	}

	scopes := ScopeAges(prs)
	if got, want := scopeOrder(scopes), []string{"beta", "alpha"}; !slices.Equal(got, want) {
		t.Fatalf("ScopeAges() order = %v, want %v", got, want)
	}
	if got, want := scopes[1].NewestPRAge, TimeAgo(summariesNow.Sub(*summaryTime(3)), 7); got != want {
		t.Errorf("alpha NewestPRAge = %q, want %q (age of newest created pull request)", got, want)
	}
}

func TestScopeAgesBreaksRecencyTiesByName(t *testing.T) {
	prs := []StampedPullRequest{
		scopeSummaryPR("zulu", 4, 4),
		scopeSummaryPR("alpha", 4, 4),
	}

	want := []string{"alpha", "zulu"}
	if got := scopeOrder(ScopeAges(prs)); !slices.Equal(got, want) {
		t.Errorf("ScopeAges() tie order = %v, want %v", got, want)
	}
}

func contributorSummaryPR(name string, createdDay, updatedDay int) StampedPullRequest {
	createdAt := summaryTime(createdDay)
	updatedAt := summaryTime(updatedDay)
	return StampedPullRequest{
		PullRequest:  PullRequest{CreatedAt: createdAt, UpdatedAt: updatedAt},
		TimeOpen:     summariesNow.Sub(*createdAt),
		DaysOpen:     int(summariesNow.Sub(*createdAt).Hours() / 24),
		Contributors: []string{name},
	}
}

func TestContributorActivitySortsByNewestCreationTime(t *testing.T) {
	prs := []StampedPullRequest{
		contributorSummaryPR("old", 1, 1),
		contributorSummaryPR("new", 5, 5),
		contributorSummaryPR("middle", 3, 3),
	}

	want := []string{"new", "middle", "old"}
	if got := contributorOrder(ContributorActivity(prs)); !slices.Equal(got, want) {
		t.Errorf("ContributorActivity() order = %v, want %v", got, want)
	}
}

func TestContributorActivityBreaksRecencyTiesByName(t *testing.T) {
	prs := []StampedPullRequest{
		contributorSummaryPR("zulu", 4, 4),
		contributorSummaryPR("alpha", 4, 4),
	}

	want := []string{"alpha", "zulu"}
	if got := contributorOrder(ContributorActivity(prs)); !slices.Equal(got, want) {
		t.Errorf("ContributorActivity() tie order = %v, want %v", got, want)
	}
}
