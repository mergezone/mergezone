package components

import (
	"slices"

	"merge/internal/model"
)

// selectedScopesFirst moves the scopes named in selected to the front of
// scopeSummaries, preserving the relative order within both groups. Keeping
// selected scopes at the front makes the active selection easy to find without
// hiding the other scopes, which stay available for multi-selection.
func selectedScopesFirst(scopeSummaries []model.ScopeInfo, selected []string) []model.ScopeInfo {
	if len(selected) == 0 {
		return scopeSummaries
	}

	first := make([]model.ScopeInfo, 0, len(scopeSummaries))
	rest := make([]model.ScopeInfo, 0, len(scopeSummaries))
	for _, scope := range scopeSummaries {
		if slices.Contains(selected, scope.Name) {
			first = append(first, scope)
		} else {
			rest = append(rest, scope)
		}
	}
	return append(first, rest...)
}

// selectedContributorsFirst is selectedScopesFirst for the contributor list.
func selectedContributorsFirst(contributorSummaries []model.ContributorInfo, selected []string) []model.ContributorInfo {
	if len(selected) == 0 {
		return contributorSummaries
	}

	first := make([]model.ContributorInfo, 0, len(contributorSummaries))
	rest := make([]model.ContributorInfo, 0, len(contributorSummaries))
	for _, contributor := range contributorSummaries {
		if slices.Contains(selected, contributor.Name) {
			first = append(first, contributor)
		} else {
			rest = append(rest, contributor)
		}
	}
	return append(first, rest...)
}
