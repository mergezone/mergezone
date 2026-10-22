package provider

import (
	"slices"

	"merge/internal/model"
)

// FilterPullRequests applies the query's scope, contributor, and status
// selections to stampedPullRequests. Values within one dimension are OR-ed
// together; the dimensions combine with AND.
func FilterPullRequests(stampedPullRequests []model.StampedPullRequest, query RepositoryQuery) []model.StampedPullRequest {
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
		// A pull request matches the status dimension once, even when several
		// of its statuses are selected.
		match := func(i int) {
			if matched[i] {
				return
			}
			matchesStatus = append(matchesStatus, filteredPullRequests[i])
			matched[i] = true
		}

		for _, status := range query.Status {
			if status == "merged" {
				for i, pr := range filteredPullRequests {
					if pr.State == model.Merged {
						match(i)
					}
				}
				continue
			}

			var expiryStatus model.ExpiryStatus
			switch status {
			case "fresh":
				expiryStatus = model.Fresh
			case "stale":
				expiryStatus = model.Stale
			case "expired":
				expiryStatus = model.Expired
			default:
				continue
			}

			for i, pr := range filteredPullRequests {
				// Merged pull requests are their own status; fresh, stale, and
				// expired describe open ones. Excluding merged here keeps a
				// status filter in step with the overview tile counts.
				if pr.State != model.Merged && pr.ExpiryStatus == expiryStatus {
					match(i)
				}
			}
		}
		filteredPullRequests = matchesStatus
	}

	return filteredPullRequests
}
