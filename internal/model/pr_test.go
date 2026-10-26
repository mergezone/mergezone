package model

import (
	"testing"
	"time"
)

func TestPullRequestStatePrioritizesClosedOverDraft(t *testing.T) {
	closedAt := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	pr := PullRequest{IsDraft: true, ClosedAt: &closedAt}
	if got := pr.State(); got != Closed {
		t.Errorf("State() = %q, want %q", got, Closed)
	}
}

func TestColonScopeExtractionSupportsCommaSeparatedScopes(t *testing.T) {
	pr := PullRequest{Title: "gtk,opengl: reuse GTK’s EGL display when possible"}
	got := pr.Scopes()
	want := []string{"gtk", "opengl"}
	if len(got) != len(want) {
		t.Fatalf("Scopes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Scopes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSubscopesParsedIntoMultiScope(t *testing.T) {
	pr := PullRequest{Title: "terminal/kitty: accept shared memory names without a leading slash"}
	got := pr.Scopes()
	want := []string{"terminal", "kitty"}
	if len(got) != len(want) {
		t.Fatalf("Scopes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Scopes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStampAppliesScopeSupersession(t *testing.T) {
	createdAt := func(day int) *time.Time {
		value := time.Date(2025, time.January, day, 0, 0, 0, 0, time.UTC)
		return &value
	}

	tests := []struct {
		name string
		prs  []PullRequest
		want []ExpiryStatus
	}{
		{
			name: "newer unmerged pull request in same scope makes older pull request stale",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): earlier", CreatedAt: createdAt(1)},
				{Number: 2, Title: "fix(backend): later", CreatedAt: createdAt(2)},
			},
			want: []ExpiryStatus{Stale, Fresh},
		},
		{
			name: "newer merged pull request in same scope makes older pull request expired",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): earlier", CreatedAt: createdAt(1)},
				{Number: 2, Title: "fix(backend): later", CreatedAt: createdAt(2), MergedAt: createdAt(3)},
			},
			want: []ExpiryStatus{Expired, Fresh},
		},
		{
			name: "different scopes do not supersede",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): earlier", CreatedAt: createdAt(1)},
				{Number: 2, Title: "fix(frontend): later", CreatedAt: createdAt(2)},
			},
			want: []ExpiryStatus{Fresh, Fresh},
		},
		{
			name: "only older pull requests are superseded",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): earlier", CreatedAt: createdAt(2)},
				{Number: 2, Title: "fix(backend): later", CreatedAt: createdAt(1)},
			},
			want: []ExpiryStatus{Fresh, Stale},
		},
		{
			name: "newest unmerged pull request determines status over older merged successor",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): earlier", CreatedAt: createdAt(1)},
				{Number: 2, Title: "fix(backend): merged", CreatedAt: createdAt(2), MergedAt: createdAt(3)},
				{Number: 3, Title: "fix(backend): newest", CreatedAt: createdAt(4)},
			},
			want: []ExpiryStatus{Stale, Fresh, Fresh},
		},
		{
			name: "newest merged pull request determines status over older unmerged successor",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): earlier", CreatedAt: createdAt(1)},
				{Number: 2, Title: "fix(backend): unmerged", CreatedAt: createdAt(2)},
				{Number: 3, Title: "fix(backend): newest merged", CreatedAt: createdAt(4), MergedAt: createdAt(5)},
			},
			want: []ExpiryStatus{Expired, Expired, Fresh},
		},
		{
			name: "overlap in any parsed scope supersedes",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend,api): earlier", CreatedAt: createdAt(1)},
				{Number: 2, Title: "fix(api): later", CreatedAt: createdAt(2)},
			},
			want: []ExpiryStatus{Stale, Fresh},
		},
		{
			name: "older merged pull request status is not rewritten",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): merged", CreatedAt: createdAt(1), MergedAt: createdAt(2)},
				{Number: 2, Title: "fix(backend): later", CreatedAt: createdAt(3)},
			},
			want: []ExpiryStatus{Fresh, Fresh},
		},
		{
			name: "equal creation times do not count as newer",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): first", CreatedAt: createdAt(1)},
				{Number: 2, Title: "fix(backend): tied", CreatedAt: createdAt(1)},
			},
			want: []ExpiryStatus{Fresh, Fresh},
		},
		{
			name: "closed unmerged pull request remains expired",
			prs: []PullRequest{
				{Number: 1, Title: "feat(backend): closed", CreatedAt: createdAt(1), ClosedAt: createdAt(2)},
				{Number: 2, Title: "fix(backend): later", CreatedAt: createdAt(3)},
			},
			want: []ExpiryStatus{Expired, Fresh},
		},
		{
			name: "pull requests without scopes do not supersede each other",
			prs: []PullRequest{
				{Number: 1, Title: "earlier", CreatedAt: createdAt(1)},
				{Number: 2, Title: "later", CreatedAt: createdAt(2)},
			},
			want: []ExpiryStatus{Fresh, Fresh},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Stamp(test.prs, time.Date(2025, time.January, 5, 0, 0, 0, 0, time.UTC))
			if len(got) != len(test.want) {
				t.Fatalf("Stamp() returned %d pull requests, want %d", len(got), len(test.want))
			}
			for i, want := range test.want {
				if got[i].ExpiryStatus != want {
					t.Errorf("pull request #%d status = %q, want %q", got[i].Number, got[i].ExpiryStatus, want)
				}
			}
		})
	}
}

func TestStampRecordsSupersedingPullRequest(t *testing.T) {
	createdAt := func(day int) *time.Time {
		value := time.Date(2025, time.January, day, 0, 0, 0, 0, time.UTC)
		return &value
	}
	now := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)

	t.Run("stale pull request references the newer unmerged pull request", func(t *testing.T) {
		got := Stamp([]PullRequest{
			{Number: 1, Title: "feat(backend): earlier", URL: "https://example.com/pull/1", CreatedAt: createdAt(1)},
			{Number: 2, Title: "fix(backend): later", URL: "https://example.com/pull/2", CreatedAt: createdAt(2)},
		}, now)

		if got[0].SupersededBy == nil {
			t.Fatal("expected a superseding pull request, got nil")
		}
		if got[0].SupersededBy.Number != 2 || got[0].SupersededBy.URL != "https://example.com/pull/2" || got[0].SupersededBy.Merged {
			t.Errorf("SupersededBy = %+v, want number 2, URL and merged false", *got[0].SupersededBy)
		}
		if got[1].SupersededBy != nil {
			t.Errorf("newest pull request should not be superseded, got %+v", *got[1].SupersededBy)
		}
	})

	t.Run("expired pull request references the newer merged pull request", func(t *testing.T) {
		got := Stamp([]PullRequest{
			{Number: 1, Title: "feat(backend): earlier", URL: "https://example.com/pull/1", CreatedAt: createdAt(1)},
			{Number: 2, Title: "fix(backend): later", URL: "https://example.com/pull/2", CreatedAt: createdAt(2), MergedAt: createdAt(3)},
		}, now)

		if got[0].SupersededBy == nil {
			t.Fatal("expected a superseding pull request, got nil")
		}
		if got[0].SupersededBy.Number != 2 || !got[0].SupersededBy.Merged {
			t.Errorf("SupersededBy = %+v, want number 2 and merged true", *got[0].SupersededBy)
		}
	})

	t.Run("age-based expiry has no superseding pull request", func(t *testing.T) {
		got := Stamp([]PullRequest{
			{Number: 1, Title: "feat(backend): ancient", CreatedAt: createdAt(1)},
		}, time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC))
		if got[0].ExpiryStatus != Expired {
			t.Fatalf("ExpiryStatus = %q, want %q", got[0].ExpiryStatus, Expired)
		}
		if got[0].SupersededBy != nil {
			t.Errorf("SupersededBy = %+v, want nil", *got[0].SupersededBy)
		}
	})
}

func TestScopeSupersessionOverridesAgeBasedExpiry(t *testing.T) {
	createdAt := func(year int, month time.Month, day int) *time.Time {
		value := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		return &value
	}
	prs := []PullRequest{
		{Number: 1, Title: "feat(backend): old", CreatedAt: createdAt(2025, time.January, 1)},
		{Number: 2, Title: "fix(backend): newer", CreatedAt: createdAt(2025, time.February, 20)},
	}

	got := Stamp(prs, time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC))
	if got[0].ExpiryStatus != Stale {
		t.Errorf("older pull request status = %q, want %q even though it exceeds the age expiry threshold", got[0].ExpiryStatus, Stale)
	}
	if got[1].ExpiryStatus != Fresh {
		t.Errorf("newest pull request status = %q, want %q", got[1].ExpiryStatus, Fresh)
	}
}

func TestTimeAgoPartsDistinguishesMinutesFromMonths(t *testing.T) {
	minuteNum, minuteSuffix := TimeAgoParts(21*time.Minute, 0)
	if minuteNum != "21" || minuteSuffix != "m" {
		t.Errorf("TimeAgoParts(21m) = (%q, %q), want (\"21\", \"m\")", minuteNum, minuteSuffix)
	}

	monthNum, monthSuffix := TimeAgoParts(90*24*time.Hour, 90)
	if monthNum != "3" || monthSuffix != "mo" {
		t.Errorf("TimeAgoParts(90d) = (%q, %q), want (\"3\", \"mo\")", monthNum, monthSuffix)
	}

	if got, want := TimeAgoLong(21*time.Minute, 0), "21 minutes"; got != want {
		t.Errorf("TimeAgoLong(21m) = %q, want %q", got, want)
	}
	if got, want := TimeAgoLong(90*24*time.Hour, 90), "3 months"; got != want {
		t.Errorf("TimeAgoLong(90d) = %q, want %q", got, want)
	}
}

func TestScopeSupersessionHandlesMissingCreationTimes(t *testing.T) {
	prs := []PullRequest{
		{Number: 1, Title: "feat(backend): missing date"},
		{Number: 2, Title: "fix(backend): dated", CreatedAt: func() *time.Time {
			value := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
			return &value
		}()},
	}

	got := Stamp(prs, time.Date(2025, time.January, 5, 0, 0, 0, 0, time.UTC))
	if len(got) != 2 {
		t.Fatalf("Stamp() returned %d pull requests, want 2", len(got))
	}
	if got[0].ExpiryStatus != Fresh || got[1].ExpiryStatus != Fresh {
		t.Errorf("statuses for pull requests with missing creation time = (%q, %q), want (Fresh, Fresh)", got[0].ExpiryStatus, got[1].ExpiryStatus)
	}
}

func TestStampScopeSupersessionIsIndependentOfInputOrder(t *testing.T) {
	createdAt := func(day int) *time.Time {
		value := time.Date(2025, time.January, day, 0, 0, 0, 0, time.UTC)
		return &value
	}
	now := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	pullRequests := []PullRequest{
		{Number: 1, Title: "feat(backend,api): earlier", CreatedAt: createdAt(1)},
		{Number: 2, Title: "fix(api): merged", CreatedAt: createdAt(2), MergedAt: createdAt(3)},
		{Number: 3, Title: "fix(backend): newest", CreatedAt: createdAt(4)},
		{Number: 4, Title: "feat(frontend): unrelated", CreatedAt: createdAt(5)},
	}

	want := map[int]ExpiryStatus{
		1: Stale, // Newest matching successor is unmerged.
		2: Fresh,
		3: Fresh,
		4: Fresh,
	}
	orders := [][]int{
		{0, 1, 2, 3},
		{3, 2, 1, 0},
		{1, 3, 0, 2},
	}
	for _, order := range orders {
		ordered := make([]PullRequest, len(order))
		for i, index := range order {
			ordered[i] = pullRequests[index]
		}

		for _, stamped := range Stamp(ordered, now) {
			if stamped.ExpiryStatus != want[stamped.Number] {
				t.Errorf("input order %v: pull request #%d status = %q, want %q", order, stamped.Number, stamped.ExpiryStatus, want[stamped.Number])
			}
		}
	}
}
