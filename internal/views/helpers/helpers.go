package helpers

import (
	"merge/internal/model"
	"strings"
)

func GetStatusClass(pr model.StampedPullRequest) string {
	switch pr.State {
	case model.Merged:
		return "fa-code-merge special"
	}
	switch pr.ExpiryStatus {
	case model.Expired:
		return "fa-skull error"
	case model.Stale:
		return "fa-leaf warn"
	case model.Fresh:
		return "fa-seedling ok"
	default:
		return "fa-eye-slash"
	}
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
