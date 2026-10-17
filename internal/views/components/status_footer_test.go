package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestStatusFooterProvidesTimingTargets(t *testing.T) {
	var output bytes.Buffer
	if err := StatusFooter("abc123").Render(context.Background(), &output); err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		"Commit <strong>abc123</strong>",
		"data-status-gather",
		"data-status-logic",
		"data-status-template",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("status footer output does not contain %q: %s", expected, output.String())
		}
	}
}
