package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"rune/internal/runeruntime"
)

var markerPattern = regexp.MustCompile(`msg-\d+`)

// sizeLimitedSummarizer returns a context-limit error when the rendered
// transcript carries more than maxMarkers messages, and otherwise "summarizes"
// by echoing the message markers it saw — so a successful summary records exactly
// which messages it covered.
type sizeLimitedSummarizer struct {
	maxMarkers int
	calls      int32
}

func (p *sizeLimitedSummarizer) StreamCompletion(_ context.Context, request runeruntime.CompletionRequest) (<-chan runeruntime.StreamEvent, error) {
	atomic.AddInt32(&p.calls, 1)
	text := request.Messages[len(request.Messages)-1].Content
	markers := markerPattern.FindAllString(text, -1)
	ch := make(chan runeruntime.StreamEvent, 2)
	if len(markers) > p.maxMarkers {
		ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventError, Error: "context length exceeded"}
		close(ch)
		return ch, nil
	}
	ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventText, Content: strings.Join(markers, " ")}
	ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventDone}
	close(ch)
	return ch, nil
}

type errorSummarizer struct {
	message string
	calls   int32
}

func (p *errorSummarizer) StreamCompletion(_ context.Context, _ runeruntime.CompletionRequest) (<-chan runeruntime.StreamEvent, error) {
	atomic.AddInt32(&p.calls, 1)
	ch := make(chan runeruntime.StreamEvent, 1)
	ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventError, Error: p.message}
	close(ch)
	return ch, nil
}

// compressingSummarizer fails on more than maxMarkers messages but returns a
// SHORT marker-free summary, so two partial summaries combine into something that
// fits — modelling real summarization that shrinks its input.
type compressingSummarizer struct {
	maxMarkers int
	calls      int32
}

func (p *compressingSummarizer) StreamCompletion(_ context.Context, request runeruntime.CompletionRequest) (<-chan runeruntime.StreamEvent, error) {
	atomic.AddInt32(&p.calls, 1)
	text := request.Messages[len(request.Messages)-1].Content
	ch := make(chan runeruntime.StreamEvent, 2)
	if len(markerPattern.FindAllString(text, -1)) > p.maxMarkers {
		ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventError, Error: "context length exceeded"}
		close(ch)
		return ch, nil
	}
	ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventText, Content: "S"}
	ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventDone}
	close(ch)
	return ch, nil
}

func TestSummarizeWithFallbackReSummarizesPartialsIntoOne(t *testing.T) {
	messages := make([]runeruntime.Message, 4)
	for i := range messages {
		messages[i] = runeruntime.Message{Role: runeruntime.MessageRoleUser, Content: fmt.Sprintf("msg-%d body", i)}
	}
	provider := &compressingSummarizer{maxMarkers: 2}

	summary, err := summarizeWithFallback(context.Background(), provider, messages, nil)
	if err != nil {
		t.Fatalf("summarizeWithFallback failed: %v", err)
	}
	// The two chunk summaries ("S" / "S") are re-summarized into ONE unit, not
	// returned as the joined "S\n\nS" blob — so a later compaction can shrink it.
	if strings.Contains(summary, "\n\n") {
		t.Fatalf("expected a single re-summarized result, got a joined blob: %q", summary)
	}
	if summary != "S" {
		t.Fatalf("summary = %q, want the reduced %q", summary, "S")
	}
}

func TestSummarizeWithFallbackChunksOnContextLimit(t *testing.T) {
	const n = 8
	messages := make([]runeruntime.Message, n)
	for i := range messages {
		messages[i] = runeruntime.Message{Role: runeruntime.MessageRoleUser, Content: fmt.Sprintf("msg-%d some content", i)}
	}
	// The summarizer can only handle 2 messages per call, so the 8-message slice
	// must be split recursively until each chunk fits.
	provider := &sizeLimitedSummarizer{maxMarkers: 2}

	summary, err := summarizeWithFallback(context.Background(), provider, messages, nil)
	if err != nil {
		t.Fatalf("summarizeWithFallback failed: %v", err)
	}
	for i := 0; i < n; i++ {
		if !strings.Contains(summary, fmt.Sprintf("msg-%d", i)) {
			t.Fatalf("combined summary missing msg-%d: %q", i, summary)
		}
	}
	if got := atomic.LoadInt32(&provider.calls); got < 2 {
		t.Fatalf("expected multiple calls from splitting, got %d", got)
	}
}

func TestSummarizeWithFallbackPropagatesNonContextErrors(t *testing.T) {
	provider := &errorSummarizer{message: "auth error: invalid key"}
	_, err := summarizeWithFallback(context.Background(), provider, []runeruntime.Message{
		{Role: runeruntime.MessageRoleUser, Content: "msg-0"},
		{Role: runeruntime.MessageRoleUser, Content: "msg-1"},
	}, nil)
	if err == nil {
		t.Fatal("expected a non-context-limit error to propagate")
	}
	if got := atomic.LoadInt32(&provider.calls); got != 1 {
		t.Fatalf("a non-context error must not trigger splitting/retry, calls=%d", got)
	}
}

func TestSummarizeWithFallbackSingleMessageContextLimitSurfaces(t *testing.T) {
	// A single message that still won't fit can't be split further → error surfaces.
	provider := &sizeLimitedSummarizer{maxMarkers: 0}
	_, err := summarizeWithFallback(context.Background(), provider, []runeruntime.Message{
		{Role: runeruntime.MessageRoleUser, Content: "msg-0 too big"},
	}, nil)
	if err == nil {
		t.Fatal("expected the context-limit error to surface for an unsplittable single message")
	}
}

// usageReportingSummarizer emits a usage event so a test can assert the
// summarizer's token cost is forwarded to OnUsage.
type usageReportingSummarizer struct{}

func (usageReportingSummarizer) StreamCompletion(_ context.Context, _ runeruntime.CompletionRequest) (<-chan runeruntime.StreamEvent, error) {
	ch := make(chan runeruntime.StreamEvent, 3)
	ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventText, Content: "summary"}
	ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventUsage, Usage: runeruntime.Usage{PromptTokens: 100, CompletionTokens: 20}}
	ch <- runeruntime.StreamEvent{Type: runeruntime.StreamEventDone}
	return ch, nil
}

func TestSummarizeForwardsUsageButNotText(t *testing.T) {
	// Compaction must stay invisible to the user (no OnText), but its token cost
	// MUST be counted, so OnUsage has to fire for the summarizer call.
	var got runeruntime.Usage
	var calls int
	summary, err := summarizeWithFallback(context.Background(), usageReportingSummarizer{}, []runeruntime.Message{
		{Role: runeruntime.MessageRoleUser, Content: "hello"},
	}, func(u runeruntime.Usage) { calls++; got = u })
	if err != nil {
		t.Fatalf("summarize failed: %v", err)
	}
	if summary != "summary" {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if calls != 1 {
		t.Fatalf("expected OnUsage to fire once, got %d", calls)
	}
	if got.PromptTokens != 100 || got.CompletionTokens != 20 {
		t.Fatalf("unexpected forwarded usage: %#v", got)
	}
}
