package helpers

import (
	"merge/internal/model"
	"strings"
)

func GetStatusClass(pr model.StampedPullRequest) string {
	switch pr.State {
	case model.Merged:
		return "fa-code-merge text-accent"
	case model.Closed:
		return "fa-ban"
	}
	switch pr.ExpiryStatus {
	case model.Expired:
		return "fa-skull text-error"
	case model.Stale:
		return "fa-leaf text-warning"
	case model.Fresh:
		return "fa-seedling text-success"
	default:
		return "fa-eye-slash text-base-content/50"
	}
}

// StatusIconClass returns just the Font Awesome icon for a pull request's
// freshness without an accompanying color class, for contexts that want the
// icon shape only.
func StatusIconClass(pr model.StampedPullRequest) string {
	switch pr.State {
	case model.Merged:
		return "fa-code-merge"
	case model.Closed:
		return "fa-ban"
	}
	switch pr.ExpiryStatus {
	case model.Expired:
		return "fa-skull"
	case model.Stale:
		return "fa-leaf"
	case model.Fresh:
		return "fa-seedling"
	default:
		return "fa-eye-slash text-base-content/50"
	}
}

// ExpiryLabel is the freshness state shown in pull request details. Merged pull
// requests report a Merged state instead of a freshness state.
func ExpiryLabel(pr model.StampedPullRequest) string {
	if pr.State == model.Merged || pr.State == model.Closed {
		return string(pr.State)
	}
	return string(pr.ExpiryStatus)
}

// ExpiryReason explains a freshness state that was not caused by a newer pull
// request, such as a pull request that was closed without merging. It returns
// an empty string when there is no additional explanation.
func ExpiryReason(pr model.StampedPullRequest) string {
	if pr.ClosedAt != nil && pr.MergedAt == nil {
		return "closed"
	}
	return ""
}

// SupersessionReason describes why a pull request became stale or expired. It
// is the phrase following the referenced pull request number, or empty when the
// status was not caused by a newer pull request.
func SupersessionReason(pr model.StampedPullRequest) string {
	if pr.SupersededBy == nil {
		return ""
	}
	if pr.SupersededBy.Merged {
		return "is newer and merged"
	}
	return "is newer"
}

func StripQueryParam(path, key string) string {
	prefix := key + "="
	if idx := strings.Index(path, "?"+prefix); idx != -1 {
		rest := path[idx+1:]
		end := strings.Index(rest, "&")
		if end == -1 {
			return path[:idx]
		}
		return path[:idx] + rest[end:]
	}
	if idx := strings.Index(path, "&"+prefix); idx != -1 {
		rest := path[idx+1:]
		end := strings.Index(rest, "&")
		if end == -1 {
			return path[:idx]
		}
		return path[:idx] + rest[end:]
	}
	return path
}

func GetQueryParam(path, key string) string {
	if idx := strings.Index(path, "?"); idx != -1 {
		query := path[idx+1:]
		prefix := key + "="
		if qIdx := strings.Index(query, prefix); qIdx != -1 {
			rest := query[qIdx+len(prefix):]
			end := strings.Index(rest, "&")
			if end == -1 {
				return rest
			}
			return rest[:end]
		}
	}
	return ""
}

func PrLabel(count int) string {
	if count == 1 {
		return "pr"
	}
	return "pull requests"
}

func SplitAge(age string) (string, string) {
	if len(age) < 2 {
		return age, ""
	}
	return age[:len(age)-1], age[len(age)-1:]
}

func LongSuffix(num, suffix string) string {
	if num == "1" {
		switch suffix {
		case "h":
			return "hour"
		case "d":
			return "day"
		case "w":
			return "week"
		case "m":
			return "month"
		case "y":
			return "year"
		}
		return suffix
	}
	switch suffix {
	case "h":
		return "hours"
	case "d":
		return "days"
	case "w":
		return "weeks"
	case "m":
		return "months"
	case "y":
		return "years"
	default:
		return suffix
	}
}
