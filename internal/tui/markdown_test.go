package tui

import (
	"strings"
	"testing"
)

func TestEmojiShortcode(t *testing.T) {
	result := renderMarkdown(":rocket:", 80)
	if !strings.Contains(result, "🚀") {
		t.Errorf("expected rocket emoji, got: %q", result)
	}
}

func TestEmojiUnknown(t *testing.T) {
	result := renderMarkdown(":notarealemoji:", 80)
	if !strings.Contains(result, ":notarealemoji:") {
		t.Errorf("expected unknown emoji to pass through, got: %q", result)
	}
}

func TestBoldMarkdown(t *testing.T) {
	result := renderInline("hello **world** ok")
	if !strings.Contains(result, "world") {
		t.Errorf("expected bold content, got: %q", result)
	}
	if strings.Contains(result, "**") {
		t.Errorf("expected ** to be removed, got: %q", result)
	}
}

func TestItalicMarkdown(t *testing.T) {
	result := renderInline("hello *world* ok")
	if !strings.Contains(result, "world") {
		t.Errorf("expected italic content, got: %q", result)
	}
	if strings.Contains(result, "*world*") {
		t.Errorf("expected *..* to be rendered, got: %q", result)
	}
}

func TestInlineCode(t *testing.T) {
	result := renderInline("run `go test` now")
	if !strings.Contains(result, "go test") {
		t.Errorf("expected code content, got: %q", result)
	}
}

func TestLinkRendering(t *testing.T) {
	result := renderInline("see https://example.com for info")
	if !strings.Contains(result, "example.com") {
		t.Errorf("expected link content, got: %q", result)
	}
}

func TestCodeBlockRendering(t *testing.T) {
	result := renderMarkdown("```\nhello\n```", 80)
	if !strings.Contains(result, "hello") {
		t.Errorf("expected code block content, got: %q", result)
	}
}

func TestPlainTextPassthrough(t *testing.T) {
	text := "just a normal message"
	result := renderMarkdown(text, 80)
	if !strings.Contains(result, text) {
		t.Errorf("expected plain text to pass through, got: %q", result)
	}
}
