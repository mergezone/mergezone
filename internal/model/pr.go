package model

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

type User struct {
	Name      string `json:"login"`
	URL       string `json:"html_url"`
	AvatarURL string `json:"avatar_url"`
}

type PullRequest struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	URL       string     `json:"html_url"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	MergedAt  *time.Time `json:"merged_at"`
	IsDraft   bool       `json:"draft"`
	Author    User       `json:"user"`
	Assignees []User     `json:"assignees"`
	Reviewers []User     `json:"requested_reviewers"`
}

func (pr *PullRequest) TimeOpen(time time.Time) time.Duration {
	created := pr.CreatedAt != nil
	if !created {
		// How?
		return 0
	}

	return time.Sub(*pr.CreatedAt)
}

func (pr *PullRequest) DaysOpen(time time.Time) int {
	hrs := pr.TimeOpen(time).Hours()
	return int(hrs / 24)
}

type State string

const (
	Draft  State = "Draft"
	Open         = "Open"
	Merged       = "Merged"
	Closed       = "Closed"
)

func (pr *PullRequest) State() State {
	if pr.IsDraft {
		return Draft
	} else if pr.MergedAt != nil {
		return Merged
	} else if pr.ClosedAt != nil {
		return Closed
	}

	return Open
}

type ExpiryStatus string

const (
	Fresh   ExpiryStatus = "Fresh"
	Stale                = "Stale"
	Expired              = "Expired"
)

func (pr *PullRequest) ExpiryStatus(time time.Time) ExpiryStatus {
	if pr.ClosedAt != nil && pr.MergedAt == nil {
		return Expired
	}
	days := pr.DaysOpen(time)
	if days >= 30 {
		return Expired
	}
	if days >= 14 {
		return Stale
	}
	return Fresh
}

func split(scopes string) []string {
	result := strings.Split(scopes, ",")
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
	}
	return result
}

func extractConventionalCommitScopes(title string) []string {
	re := regexp.MustCompile(`^\w+\(([^)]+)\)(?:!)?:`)
	matches := re.FindStringSubmatch(title)
	if len(matches) == 0 {
		return []string{}
	}

	return split(matches[1])
}

func extractSquareBracketScopes(title string) []string {
	re := regexp.MustCompile(`^\[([^\]]+)\]`)
	matches := re.FindStringSubmatch(title)
	if len(matches) == 0 {
		return []string{}
	}

	return split(matches[1])
}

func extractColonScopes(title string) []string {
	re := regexp.MustCompile(`^([a-zA-Z0-9_\-/]+):`)
	matches := re.FindStringSubmatch(title)
	if len(matches) == 0 {
		return []string{}
	}

	return split(matches[1])
}

func (pr *PullRequest) Scopes() []string {
	result := make([]string, 0)
	for _, s := range extractConventionalCommitScopes(pr.Title) {
		result = append(result, s)
	}
	for _, s := range extractSquareBracketScopes(pr.Title) {
		result = append(result, s)
	}
	for _, s := range extractColonScopes(pr.Title) {
		result = append(result, s)
	}
	return result
}

func (pr *PullRequest) Contributors() []string {
	if pr.Author.Name != "" {
		return []string{pr.Author.Name}
	}
	return []string{}
}

func (pr *PullRequest) IsBot() bool {
	return strings.Contains(pr.Author.URL, "app")
}

type StampedPullRequest struct {
	PullRequest
	Time         time.Time
	State        State
	TimeOpen     time.Duration
	DaysOpen     int
	ExpiryStatus ExpiryStatus
	Type         string
	Scopes       []string
	Contributors []string
}

func (pr *PullRequest) Stamp(time time.Time) StampedPullRequest {
	spr := StampedPullRequest{
		PullRequest:  *pr,
		Time:         time,
		State:        pr.State(),
		TimeOpen:     pr.TimeOpen(time),
		DaysOpen:     pr.DaysOpen(time),
		ExpiryStatus: pr.ExpiryStatus(time),
		Scopes:       pr.Scopes(),
		Contributors: pr.Contributors(),
	}

	return spr
}

func Stamp(prs []PullRequest, time time.Time) []StampedPullRequest {
	stamped := make([]StampedPullRequest, 0, len(prs))
	for _, pr := range prs {
		if !pr.IsBot() {
			stamped = append(stamped, pr.Stamp(time))
		}
	}
	return stamped
}

func StampNow(prs []PullRequest) []StampedPullRequest {
	stamped := make([]StampedPullRequest, 0, len(prs))
	now := time.Now()
	for _, pr := range prs {
		if !pr.IsBot() {
			stamped = append(stamped, pr.Stamp(now))
		}
	}
	return stamped
}

type ExpiryCounts struct {
	MergedCount  int
	FreshCount   int
	StaleCount   int
	ExpiredCount int
}

func (c ExpiryCounts) TotalPullRequests() int {
	return c.FreshCount + c.StaleCount + c.ExpiredCount + c.MergedCount
}

func PullRequestExpiryCounts(pullRequests []StampedPullRequest) ExpiryCounts {
	merged := 0
	fresh := 0
	stale := 0
	expired := 0

	for _, pr := range pullRequests {
		if pr.State == Merged {
			merged += 1
			continue
		}

		switch pr.ExpiryStatus {
		case Fresh:
			fresh += 1
		case Expired:
			expired += 1
		case Stale:
			stale += 1
		}
	}

	return ExpiryCounts{
		MergedCount:  merged,
		FreshCount:   fresh,
		StaleCount:   stale,
		ExpiredCount: expired,
	}
}

func ScopeCounts(prs []StampedPullRequest) map[string]ExpiryCounts {
	scopes := make(map[string][]StampedPullRequest)

	for _, pr := range prs {
		for _, scope := range pr.Scopes {
			scopes[scope] = append(scopes[scope], pr)
		}
	}

	counts := make(map[string]ExpiryCounts)

	for scope, prs := range scopes {
		counts[scope] = PullRequestExpiryCounts(prs)
	}

	return counts
}

type ScopeInfo struct {
	Name        string
	Counts      ExpiryCounts
	NewestPRAge string
}

func (s ScopeInfo) TotalPullRequests() int {
	return s.Counts.TotalPullRequests()
}

func TimeAgo(open time.Duration, days int) string {
	num, suffix := TimeAgoParts(open, days)
	return num + suffix
}

func TimeAgoLong(open time.Duration, days int) string {
	num, suffix := TimeAgoParts(open, days)
	if num == "1" {
		switch suffix {
		case "h":
			return num + " hour"
		case "d":
			return num + " day"
		case "w":
			return num + " week"
		case "m":
			return num + " month"
		case "y":
			return num + " year"
		}
		return num + suffix
	}
	switch suffix {
	case "h":
		return num + " hours"
	case "d":
		return num + " days"
	case "w":
		return num + " weeks"
	case "m":
		return num + " months"
	case "y":
		return num + " years"
	}
	return num + suffix
}

func TimeAgoParts(open time.Duration, days int) (string, string) {
	if days < 1 {
		hrs := int(math.Floor(open.Hours()))
		if hrs < 1 {
			mins := int(math.Floor(open.Minutes()))
			return fmt.Sprintf("%d", mins), "m"
		}
		return fmt.Sprintf("%d", hrs), "h"
	}

	if days < 7 {
		return fmt.Sprintf("%d", days), "d"
	}

	weeks := days / 7
	if weeks < 4 {
		return fmt.Sprintf("%d", weeks), "w"
	}

	months := weeks / 4
	if months < 12 {
		return fmt.Sprintf("%d", months), "m"
	}

	years := months / 12
	return fmt.Sprintf("%d", years), "y"
}

func ScopeAges(prs []StampedPullRequest) []ScopeInfo {
	type scopeData struct {
		prs        []StampedPullRequest
		newestTime time.Time
	}

	scopeMap := make(map[string]*scopeData)

	for _, pr := range prs {
		for _, scope := range pr.Scopes {
			if scopeMap[scope] == nil {
				scopeMap[scope] = &scopeData{}
			}
			scopeMap[scope].prs = append(scopeMap[scope].prs, pr)
			if pr.UpdatedAt != nil && (scopeMap[scope].newestTime.IsZero() || pr.UpdatedAt.After(scopeMap[scope].newestTime)) {
				scopeMap[scope].newestTime = *pr.UpdatedAt
			}
		}
	}

	type scopeEntry struct {
		name       string
		counts     ExpiryCounts
		ageStr     string
		newestTime time.Time
	}

	var entries []scopeEntry
	for name, data := range scopeMap {
		counts := PullRequestExpiryCounts(data.prs)
		ageStr := ""
		for _, pr := range data.prs {
			if pr.UpdatedAt != nil && pr.UpdatedAt.Equal(data.newestTime) {
				ageStr = TimeAgo(pr.TimeOpen, pr.DaysOpen)
				break
			}
		}
		entries = append(entries, scopeEntry{
			name:       name,
			counts:     counts,
			ageStr:     ageStr,
			newestTime: data.newestTime,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].newestTime.After(entries[j].newestTime)
	})

	result := make([]ScopeInfo, len(entries))
	for i, e := range entries {
		result[i] = ScopeInfo{
			Name:        e.name,
			Counts:      e.counts,
			NewestPRAge: e.ageStr,
		}
	}

	return result
}

type ContributorInfo struct {
	Name        string
	AvatarURL   string
	URL         string
	Counts      ExpiryCounts
	NewestPRAge string
}

func (c ContributorInfo) TotalPullRequests() int {
	return c.Counts.TotalPullRequests()
}

func ContributorActivity(prs []StampedPullRequest) []ContributorInfo {
	type contrib struct {
		ContributorInfo
		prs        []StampedPullRequest
		newestTime time.Time
	}

	m := make(map[string]*contrib)
	for _, pr := range prs {
		for _, name := range pr.Contributors {
			c, ok := m[name]
			if !ok {
				c = &contrib{
					ContributorInfo: ContributorInfo{
						Name:      name,
						AvatarURL: pr.Author.AvatarURL,
						URL:       pr.Author.URL,
					},
				}
				m[name] = c
			}
			c.prs = append(c.prs, pr)
			if pr.UpdatedAt != nil && pr.UpdatedAt.After(c.newestTime) {
				c.newestTime = *pr.UpdatedAt
				c.NewestPRAge = TimeAgo(pr.TimeOpen, pr.DaysOpen)
			}
		}
	}

	entries := make([]*contrib, 0, len(m))
	for _, c := range m {
		c.Counts = PullRequestExpiryCounts(c.prs)
		entries = append(entries, c)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].newestTime.After(entries[j].newestTime)
	})

	result := make([]ContributorInfo, len(entries))
	for i, e := range entries {
		result[i] = e.ContributorInfo
	}
	return result
}
