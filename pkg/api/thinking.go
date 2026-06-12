package api

import (
	"os"
	"strings"
)

// EmberShowThinkingEnv is the NAMED env flag that controls whether a thinking
// model's reasoning is surfaced (to stderr / the thinking sink). It defaults to
// OFF: stdout always carries the final answer only. Mirrors the Rust reference's
// separate-section treatment of thinking content. Truthy values: 1/true/yes/on.
const EmberShowThinkingEnv = "EMBER_SHOW_THINKING"

const (
	thinkOpenTag  = "<think>"
	thinkCloseTag = "</think>"
)

// ShowThinking reports whether thinking content should be made visible, reading
// the EMBER_SHOW_THINKING env flag. Off by default.
func ShowThinking() bool {
	return envFlagEnabled(os.Getenv(EmberShowThinkingEnv))
}

// envFlagEnabled interprets a string env value as a boolean flag. Empty/unset is
// false; "1", "true", "yes", "on" (case-insensitive) are true.
func envFlagEnabled(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// thinkSplitter separates a model's reasoning from its final answer in a single
// streaming pass. Some Ollama thinking models (qwen3, deepseek-r1) inline a
// LEADING `<think>...</think>` block in message.content; this splitter routes
// the contents of that well-formed leading block to the "thinking" stream and
// everything else to the "answer" stream. It parses properly (state machine over
// the tag boundaries) rather than blindly deleting, so legitimate `<` content or
// a non-leading "<think>" mention is never mangled.
//
// It is fed content deltas via push() as they stream in (holding back only the
// few characters that could be a partial tag boundary) and finalized with
// flush(). Structured `message.thinking` (the think:true path) is handled by the
// caller and does not flow through this splitter.
type thinkSplitter struct {
	state   splitterState
	pending string
}

type splitterState int

const (
	// splitterUndecided: still determining whether content opens with a
	// well-formed leading <think> block.
	splitterUndecided splitterState = iota
	// splitterThinking: inside the leading think block; route to thinking until
	// the closing tag.
	splitterThinking
	// splitterAnswer: the answer body; pass everything through.
	splitterAnswer
)

// push consumes a streamed content delta and returns any newly-resolved answer
// and thinking text. It may return empty strings while it holds back a partial
// tag boundary; the held text is resolved by a later push or by flush.
func (s *thinkSplitter) push(delta string) (answer, thinking string) {
	s.pending += delta
	var ans, think strings.Builder
	for {
		switch s.state {
		case splitterUndecided:
			trimmed := strings.TrimLeft(s.pending, " \t\r\n")
			if trimmed == "" {
				// Only leading whitespace so far — could precede <think>; wait.
				return ans.String(), think.String()
			}
			if strings.HasPrefix(trimmed, thinkOpenTag) {
				// Well-formed leading think block: drop leading WS + open tag,
				// then continue parsing the remainder as thinking.
				s.pending = trimmed[len(thinkOpenTag):]
				s.state = splitterThinking
				continue
			}
			if strings.HasPrefix(thinkOpenTag, trimmed) {
				// `trimmed` is a partial prefix of "<think>" — wait for more.
				return ans.String(), think.String()
			}
			// No leading think block: everything (incl. leading WS) is answer.
			ans.WriteString(s.pending)
			s.pending = ""
			s.state = splitterAnswer
			return ans.String(), think.String()

		case splitterThinking:
			if idx := strings.Index(s.pending, thinkCloseTag); idx >= 0 {
				think.WriteString(s.pending[:idx])
				s.pending = s.pending[idx+len(thinkCloseTag):]
				// Drop a single newline immediately after the close tag so the
				// answer doesn't start with a stray blank line.
				s.pending = strings.TrimPrefix(s.pending, "\n")
				s.state = splitterAnswer
				continue
			}
			// No close tag yet: emit thinking but hold back a possible partial
			// closing tag at the tail.
			hold := partialTagSuffixLen(s.pending, thinkCloseTag)
			if hold < len(s.pending) {
				think.WriteString(s.pending[:len(s.pending)-hold])
				s.pending = s.pending[len(s.pending)-hold:]
			}
			return ans.String(), think.String()

		default: // splitterAnswer
			ans.WriteString(s.pending)
			s.pending = ""
			return ans.String(), think.String()
		}
	}
}

// flush returns any text still buffered once the stream ends. Undecided leftover
// means no complete leading think block ever formed, so it is answer; an unclosed
// think block means the remainder is thinking.
func (s *thinkSplitter) flush() (answer, thinking string) {
	switch s.state {
	case splitterThinking:
		return "", s.pending
	default:
		return s.pending, ""
	}
}

// partialTagSuffixLen returns the length of the longest suffix of s that is a
// proper (non-full) prefix of tag — i.e. the number of trailing chars that could
// be the start of a split tag and must be held back. Returns 0 when none match.
func partialTagSuffixLen(s, tag string) int {
	maxLen := min(len(tag)-1, len(s))
	for n := maxLen; n > 0; n-- {
		if strings.HasPrefix(tag, s[len(s)-n:]) {
			return n
		}
	}
	return 0
}

// stripLeadingThinkBlock applies the splitter to a complete content string and
// returns (answer, thinking). It is the non-streaming convenience used by the
// single-shot SendMessage path; the streaming Chat path drives the splitter
// incrementally instead.
func stripLeadingThinkBlock(content string) (answer, thinking string) {
	var s thinkSplitter
	a, t := s.push(content)
	fa, ft := s.flush()
	return a + fa, t + ft
}
