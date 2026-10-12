package helpers

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/gomarkdown/markdown"
	"github.com/microcosm-cc/bluemonday"
)

var (
	prRefRe      = regexp.MustCompile(`#(\d+)\b`)
	htmlEntityRe = regexp.MustCompile(`^&(?:#[0-9]+|#x[0-9A-Fa-f]+|[A-Za-z][A-Za-z0-9]+);`)
)

func deferImages(html string) string {
	var buf strings.Builder
	buf.Grow(len(html) + 256)
	i := 0
	for {
		start := strings.Index(html[i:], "<img")
		if start == -1 {
			buf.WriteString(html[i:])
			break
		}
		start += i
		buf.WriteString(html[i:start])
		end := strings.IndexByte(html[start:], '>')
		if end == -1 {
			buf.WriteString(html[start:])
			break
		}
		end += start + 1
		tag := html[start:end]
		tag = strings.Replace(tag, " src=", " data-src=", 1)
		tag = strings.Replace(tag, "\tsrc=", "\tdata-src=", 1)
		if strings.Contains(tag, "src=") && !strings.Contains(tag, "data-src=") {
			tag = strings.Replace(tag, "src=", "data-src=", 1)
		}
		tag = `<div class="img-skeleton-wrapper">` + tag + `</div>`
		buf.WriteString(tag)
		i = end
	}
	return buf.String()
}

func linkPRRefs(html string) string {
	var buf strings.Builder
	buf.Grow(len(html) + 128)
	inAnchor := false
	codeDepth := 0

	for i := 0; i < len(html); i++ {
		c := html[i]
		if c == '<' {
			end := strings.IndexByte(html[i:], '>')
			if end == -1 {
				buf.WriteString(html[i:])
				return buf.String()
			}
			tag := html[i : i+end+1]
			buf.WriteString(tag)

			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(tag[1:len(tag)-1], "/"), "/"))
			if space := strings.IndexAny(name, " \t\r\n"); space >= 0 {
				name = name[:space]
			}
			name = strings.ToLower(name)
			closing := strings.HasPrefix(tag, "</")
			if name == "a" {
				inAnchor = !closing
			}
			if name == "code" || name == "pre" {
				if closing {
					if codeDepth > 0 {
						codeDepth--
					}
				} else if !strings.HasSuffix(strings.TrimSpace(tag), "/>") {
					codeDepth++
				}
			}
			i += end
			continue
		}
		if c == '&' {
			if entity := htmlEntityRe.FindString(html[i:]); entity != "" {
				buf.WriteString(entity)
				i += len(entity) - 1
				continue
			}
		}
		if c == '#' && !inAnchor && codeDepth == 0 && i+1 < len(html) && html[i+1] >= '0' && html[i+1] <= '9' {
			if loc := prRefRe.FindStringSubmatch(html[i:]); loc != nil {
				buf.WriteString(`<a class="pull-request-reference" data-pull-request-number="`)
				buf.WriteString(loc[1])
				buf.WriteString(`">`)
				buf.WriteString(loc[0])
				buf.WriteString(`</a>`)
				i += len(loc[0]) - 1
				continue
			}
		}
		buf.WriteByte(c)
	}
	return buf.String()
}

var (
	liUncheckedRe = regexp.MustCompile(`<li>\[ \] `)
	liCheckedRe   = regexp.MustCompile(`<li>\[[xX]\] `)
	taskBlockRe   = regexp.MustCompile(`(?s)<ul>\s*(?:<li\s+class="task[^"]*">.*?</li>\s*)+</ul>`)
)

func taskListHTML(html string) string {
	html = liCheckedRe.ReplaceAllString(html, `<li class="task done"><i class="fa-solid fa-square-check"></i><div>`)
	html = liUncheckedRe.ReplaceAllString(html, `<li class="task"><i class="fa-regular fa-square"></i><div>`)
	return taskBlockRe.ReplaceAllStringFunc(html, func(block string) string {
		block = strings.ReplaceAll(block, "<li ", "<div ")
		block = strings.ReplaceAll(block, "</li>", "</div></div>")
		block = strings.ReplaceAll(block, "<ul>", "")
		block = strings.ReplaceAll(block, "</ul>", "")
		return strings.TrimSpace(block)
	})
}

var (
	bareEmbedURLRe      = regexp.MustCompile(`(?m)^([\t \r]*)(https?://[^\s<>]+)([\t \r]*)$`)
	videoMIMEByExt      = map[string]string{".mp4": "video/mp4", ".m4v": "video/mp4", ".webm": "video/webm", ".ogv": "video/ogg", ".ogg": "video/ogg", ".mov": "video/quicktime"}
	attachmentTypeCache sync.Map
	cachedPolicy        = discussionMarkdownPolicy()
)

func discussionMarkdownPolicy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowElements("video", "source", "iframe")
	policy.AllowAttrs("controls", "playsinline").OnElements("video")
	policy.AllowAttrs("src", "poster").OnElements("video")
	policy.AllowAttrs("src").OnElements("source")
	policy.AllowAttrs("type").Matching(regexp.MustCompile(`^video/(mp4|webm|ogg|quicktime)$`)).OnElements("source")
	policy.AllowAttrs("src").Matching(regexp.MustCompile(`^https://(www\.youtube-nocookie\.com/embed/[A-Za-z0-9_-]+|player\.vimeo\.com/video/[0-9]+)$`)).OnElements("iframe")
	policy.AllowAttrs("title").OnElements("iframe")
	policy.AllowAttrs("loading").Matching(regexp.MustCompile(`^(lazy|eager)$`)).OnElements("iframe")
	policy.AllowAttrs("allowfullscreen").OnElements("iframe")
	return policy
}

func embeddedMarkdown(body string, attachmentTypes map[string]string) string {
	return bareEmbedURLRe.ReplaceAllStringFunc(body, func(line string) string {
		match := bareEmbedURLRe.FindStringSubmatch(line)
		parsed, err := url.Parse(match[2])
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return line
		}
		trimmed := strings.TrimRight(parsed.Path, ".,;:!?)]}")
		parsed.Path = trimmed
		ext := strings.ToLower(path.Ext(parsed.Path))
		indent := match[1]
		suffix := match[3]
		mediaType := attachmentTypes[match[2]]
		if mediaType == "" {
			mediaType = videoMIMEByExt[ext]
		}
		if strings.HasPrefix(mediaType, "video/") {
			return fmt.Sprintf("%s<video controls><source src=\"%s\" type=\"%s\"></video>%s", indent, parsed.String(), mediaType, suffix)
		}
		if strings.HasPrefix(mediaType, "image/") {
			return fmt.Sprintf("%s![image](%s)%s", indent, parsed.String(), suffix)
		}
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
			return fmt.Sprintf("%s![image](%s)%s", indent, parsed.String(), suffix)
		}

		host := strings.ToLower(parsed.Hostname())
		var videoID string
		switch host {
		case "youtu.be":
			videoID = strings.Trim(parsed.Path, "/")
		case "youtube.com", "www.youtube.com", "m.youtube.com":
			if parsed.Path == "/watch" {
				videoID = parsed.Query().Get("v")
			} else if strings.HasPrefix(parsed.Path, "/embed/") || strings.HasPrefix(parsed.Path, "/shorts/") {
				videoID = path.Base(parsed.Path)
			}
		}
		if regexp.MustCompile(`^[A-Za-z0-9_-]{6,}$`).MatchString(videoID) {
			return fmt.Sprintf(`%s<iframe src="https://www.youtube-nocookie.com/embed/%s" title="YouTube video" loading="lazy" allowfullscreen></iframe>%s`, indent, videoID, suffix)
		}
		if host == "vimeo.com" && regexp.MustCompile(`^[0-9]+$`).MatchString(strings.Trim(parsed.Path, "/")) {
			return fmt.Sprintf(`%s<iframe src="https://player.vimeo.com/video/%s" title="Vimeo video" loading="lazy" allowfullscreen></iframe>%s`, indent, strings.Trim(parsed.Path, "/"), suffix)
		}
		return line
	})
}

func PullRequestReferences(body string) []int {
	matches := prRefRe.FindAllStringSubmatch(body, -1)
	seen := make(map[int]struct{}, len(matches))
	refs := make([]int, 0, len(matches))
	for _, match := range matches {
		var number int
		if _, err := fmt.Sscanf(match[1], "%d", &number); err != nil {
			continue
		}
		if _, ok := seen[number]; ok {
			continue
		}
		seen[number] = struct{}{}
		refs = append(refs, number)
	}
	return refs
}

func RenderMarkdown(body string) string {
	return renderMarkdown(body, nil)
}

// RenderDiscussionMarkdown detects the media type of GitHub user-attachment URLs
// before rendering them, since these URLs do not include their file extension.
func RenderDiscussionMarkdown(body string) string {
	attachmentTypes := make(map[string]string)
	for _, match := range bareEmbedURLRe.FindAllStringSubmatch(body, -1) {
		rawURL := strings.TrimRight(match[2], ".,;:!?)]}")
		if isGitHubUserAttachment(rawURL) {
			if mediaType := detectAttachmentMediaType(rawURL); mediaType != "" {
				attachmentTypes[match[2]] = mediaType
			}
		}
	}
	return renderMarkdown(body, attachmentTypes)
}

func renderMarkdown(body string, attachmentTypes map[string]string) string {
	if body == "" {
		return ""
	}
	html := markdown.ToHTML([]byte(embeddedMarkdown(body, attachmentTypes)), nil, nil)
	sanitized := cachedPolicy.Sanitize(string(html))
	return linkPRRefs(deferImages(taskListHTML(sanitized)))
}

func isGitHubUserAttachment(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() == "github.com" && strings.HasPrefix(parsed.Path, "/user-attachments/assets/")
}

type attachmentMediaType struct {
	mediaType string
}

func detectAttachmentMediaType(rawURL string) string {
	if cached, ok := attachmentTypeCache.Load(rawURL); ok {
		return cached.(attachmentMediaType).mediaType
	}

	request, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		attachmentTypeCache.Store(rawURL, attachmentMediaType{})
		return ""
	}
	client := &http.Client{
		Timeout: 4 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		attachmentTypeCache.Store(rawURL, attachmentMediaType{})
		return ""
	}
	defer response.Body.Close()

	mediaType := ""
	if location, err := response.Location(); err == nil {
		mediaType, _, _ = mime.ParseMediaType(location.Query().Get("response-content-type"))
		if mediaType == "" {
			mediaType = mediaTypeForExtension(path.Ext(location.Path))
		}
	}
	if mediaType == "" {
		mediaType, _, _ = mime.ParseMediaType(response.Header.Get("Content-Type"))
	}
	if !isSupportedAttachmentMediaType(mediaType) {
		mediaType = ""
	}
	attachmentTypeCache.Store(rawURL, attachmentMediaType{mediaType: mediaType})
	return mediaType
}

func mediaTypeForExtension(extension string) string {
	extension = strings.ToLower(extension)
	if mediaType := videoMIMEByExt[extension]; mediaType != "" {
		return mediaType
	}
	switch extension {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	}
	return ""
}

func isSupportedAttachmentMediaType(mediaType string) bool {
	if _, ok := map[string]bool{"video/mp4": true, "video/webm": true, "video/ogg": true, "video/quicktime": true}[mediaType]; ok {
		return true
	}
	switch mediaType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp":
		return true
	default:
		return false
	}
}

func RenderMarkdownComponent(body string) templ.Component {
	if body == "" {
		return templ.NopComponent
	}
	return templ.Raw(RenderMarkdown(body))
}
