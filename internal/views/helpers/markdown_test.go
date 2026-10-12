package helpers

import (
	"strings"
	"testing"
)

func TestRenderMarkdownConvertsImageAndVideoEmbeds(t *testing.T) {
	got := RenderMarkdown(strings.Join([]string{
		"https://example.com/image.png",
		"https://example.com/clip.mp4",
		"https://youtu.be/abcDEF_1234",
		"https://vimeo.com/12345",
	}, "\n\n"))

	for _, want := range []string{
		`<img data-src="https://example.com/image.png"`,
		`<video controls=""><source src="https://example.com/clip.mp4" type="video/mp4"></video>`,
		`<iframe src="https://www.youtube-nocookie.com/embed/abcDEF_1234"`,
		`<iframe src="https://player.vimeo.com/video/12345"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderMarkdown() missing %q in output:\n%s", want, got)
		}
	}
}

func TestRenderMarkdownDetectsCRLFAttachmentLines(t *testing.T) {
	url := "https://github.com/user-attachments/assets/example-video"
	got := renderMarkdown("\r\n"+url+"\r\n", map[string]string{url: "video/mp4"})
	if !strings.Contains(got, `<video controls=""><source src="`+url+`" type="video/mp4"></video>`) {
		t.Errorf("expected extensionless video attachment to render as video, got:\n%s", got)
	}
}

func TestRenderMarkdownTransformsDetectedExtensionlessAttachments(t *testing.T) {
	videoURL := "https://github.com/user-attachments/assets/example-video"
	imageURL := "https://github.com/user-attachments/assets/example-image"
	got := renderMarkdown(videoURL+"\n\n"+imageURL, map[string]string{
		videoURL: "video/mp4",
		imageURL: "image/jpeg",
	})
	if !strings.Contains(got, `<video controls=""><source src="`+videoURL+`" type="video/mp4"></video>`) {
		t.Errorf("expected extensionless attachment to render as video, got:\\n%s", got)
	}
	if !strings.Contains(got, `<img data-src="`+imageURL+`"`) {
		t.Errorf("expected extensionless attachment to render as image, got:\\n%s", got)
	}
}

func TestRenderMarkdownPreservesQuotesAndReferencesInCodeBlocks(t *testing.T) {
	got := RenderMarkdown("```java\nvar root = mechanism.getRoot(\"demo\", 1.5, 1.5);\n// Keep PR #34 as a literal in code.\n```")
	if !strings.Contains(got, `getRoot(&#34;demo&#34;`) {
		t.Errorf("expected quoted code to remain HTML-escaped for browser rendering, got:\n%s", got)
	}
	if strings.Contains(got, `data-pull-request-number="34"`) {
		t.Errorf("did not expect PR links inside code blocks, got:\n%s", got)
	}
}

func TestRenderMarkdownSanitizesUntrustedEmbedMarkup(t *testing.T) {
	got := RenderMarkdown(`<iframe src="https://evil.example/embed/x" onload="alert(1)"></iframe><script>alert(1)</script>`)
	if strings.Contains(got, "evil.example") || strings.Contains(got, "onload") || strings.Contains(got, "<script") {
		t.Errorf("RenderMarkdown() retained unsafe embed markup: %s", got)
	}
}

func TestRenderMarkdownInlineKeepsInlineFormattingWithoutParagraph(t *testing.T) {
	got := RenderMarkdownInline("Fix `foo` and **bar**")
	want := "Fix <code>foo</code> and <strong>bar</strong>"
	if got != want {
		t.Errorf("RenderMarkdownInline() = %q, want %q", got, want)
	}
}

func TestRenderMarkdownInlineEscapesUntrustedMarkup(t *testing.T) {
	got := RenderMarkdownInline("<script>alert(1)</script> `ok`")
	if strings.Contains(got, "<script") {
		t.Errorf("RenderMarkdownInline() retained unsafe markup: %s", got)
	}
}
