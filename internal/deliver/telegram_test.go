package deliver

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunk_ShortText(t *testing.T) {
	text := "Hello, world!"
	got := chunk(text, 4000)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(got))
	}
	if got[0] != text {
		t.Fatalf("expected %q, got %q", text, got[0])
	}
}

func TestChunk_ExactLimit(t *testing.T) {
	text := strings.Repeat("a", 4000)
	got := chunk(text, 4000)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(got))
	}
}

func TestChunk_MultiLinePreservesContent(t *testing.T) {
	// Build a text whose lines each fit within limit but total exceeds it.
	limit := 20
	lines := []string{
		"line one here",    // 13 chars
		"line two here",    // 13 chars
		"line three here",  // 15 chars
		"line four here",   // 14 chars
		"line five here",   // 14 chars
	}
	text := strings.Join(lines, "\n")

	got := chunk(text, limit)

	// Each chunk must be <= limit.
	for i, c := range got {
		if len(c) > limit {
			t.Errorf("chunk %d len=%d > limit %d: %q", i, len(c), limit, c)
		}
	}

	// Must produce more than 1 chunk (total length exceeds limit).
	if len(got) <= 1 {
		t.Fatalf("expected multiple chunks for long input, got %d", len(got))
	}

	// Reassembling with "\n" must preserve original content.
	reassembled := strings.Join(got, "\n")
	if reassembled != text {
		t.Fatalf("content not preserved after reassembly:\nwant: %q\n got: %q", text, reassembled)
	}
}

func TestChunk_LongSingleLine_HardSplit(t *testing.T) {
	// A single line longer than limit must be hard-split.
	limit := 10
	line := "abcdefghijklmnopqrstuvwxyz" // 26 chars
	got := chunk(line, limit)
	if len(got) < 3 {
		t.Fatalf("expected at least 3 chunks for 26-char line with limit 10, got %d", len(got))
	}
	for i, c := range got {
		if len(c) > limit {
			t.Errorf("chunk %d len=%d > limit %d", i, len(c), limit)
		}
	}
	// Joining without separator reconstructs the line (hard-split means no newlines).
	if strings.Join(got, "") != line {
		t.Fatalf("hard-split did not preserve content: want %q got %q", line, strings.Join(got, ""))
	}
}

func TestChunk_EmptyString(t *testing.T) {
	got := chunk("", 4000)
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("expected single empty chunk, got %v", got)
	}
}

func TestChunk_LongEmojiLine_NoBrokenRunes(t *testing.T) {
	// a single line of multi-byte runes longer than the limit
	line := strings.Repeat("🟢", 50) // 50 emoji, one line, > limit chars below
	chunks := chunk(line, 10)
	for _, c := range chunks {
		if utf8.RuneCountInString(c) > 10 {
			t.Fatalf("chunk exceeds limit: %d runes", utf8.RuneCountInString(c))
		}
		if !utf8.ValidString(c) {
			t.Fatalf("chunk has broken UTF-8: %q", c)
		}
	}
	if strings.Join(chunks, "") != line {
		t.Fatalf("reassembly lost content")
	}
}
