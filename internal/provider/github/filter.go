package github

import (
	"slices"

	"merge/internal/model"
	"merge/internal/provider"
)

// filterPullRequests applies the query's scope, contributor, and status
// selections to stampedPullRequests. Values within one dimension are OR-ed
// together; the dimensions combine with AND.
func filterPullRequests(stampedPullRequests []model.StampedPullRequest, query provider.RepositoryQuery) []model.StampedPullRequest {
	filteredPullRequests := stampedPullRequests
	if query.Scope != nil {
		inScope := make([]model.StampedPullRequest, 0, len(stampedPullRequests))
		for _, pr := range stampedPullRequests {
			for _, scope := range pr.Scopes {
				if slices.Contains(query.Scope, scope) {
					inScope = append(inScope, pr)
					// A pull request matches the scope dimension once, even
					// when several of its scopes are selected.
					break
				}
			}
		}
		filteredPullRequests = inScope
	}

	if query.Contributor != nil {
		byContributor := make([]model.StampedPullRequest, 0, len(filteredPullRequests))
		for _, pr := range filteredPullRequests {
			if slices.Contains(query.Contributor, pr.Author.Name) {
				byContributor = append(byContributor, pr)
			}
		}
		filteredPullRequests = byContributor
	}

	if query.Status != nil {
		matchesStatus := make([]model.StampedPullRequest, 0, len(filteredPullRequests))
		matched := make([]bool, len(filteredPullRequests))
		for _, status := range query.Status {
			// TODO Subtle bug: ExpiryStatus and State both extend string, so no warning is thrown
			var flag model.ExpiryStatus
			switch status {
			case "merged":
				flag = model.Merged
			case "fresh":
				flag = model.Fresh
			case "stale":
				flag = model.Stale
			case "expired":
				flag = model.Expired
			}

			for i, pr := range filteredPullRequests {
				if matched[i] {
					continue
				}
				merged := flag == model.Merged && pr.State == model.Merged
				if merged || pr.ExpiryStatus == flag {
					matchesStatus = append(matchesStatus, pr)
					// A pull request matches the status dimension once, even
					// when several of its statuses are selected.
					matched[i] = true
				}
			}
		}
		filteredPullRequests = matchesStatus
	}

	return filteredPullRequests
}
