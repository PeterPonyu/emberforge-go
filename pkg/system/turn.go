package system

// TurnEngine mirrors the responsibility of QueryEngine in claude-code-src
// (claude-code-src/QueryEngine.ts:186): it owns interruptibility, accumulated
// usage, and per-session budget guardrails for the control sequence layer.
//
// The existing ControlSequenceEngine already covers:
//   - lifecycle transitions
//   - dispatch routing
//   - executing one turn
//   - persisting + reporting
//
// What QueryEngine adds on top of that — and what TurnEngine introduces here —
// is the *turn loop contract*:
//   - submit a turn
//   - track tokens / cost
//   - enforce maxTurns / maxBudget
//   - support interrupt() at any point
//
// The TS reference is a single 1320-line class; the Go translation keeps the
// concept but stays small and composable, in the spirit of the existing
// pkg/system layout (one file per concern).

import (
	"errors"
	"sync"
)

// TurnUsage is the per-turn accounting record. The names mirror the TS
// NonNullableUsage shape (input/output/cache_read/cache_creation tokens) so
// the mapping back to QueryEngine.totalUsage stays obvious.
type TurnUsage struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	CostUSD             float64
}

func (u TurnUsage) Add(other TurnUsage) TurnUsage {
	return TurnUsage{
		InputTokens:         u.InputTokens + other.InputTokens,
		OutputTokens:        u.OutputTokens + other.OutputTokens,
		CacheReadTokens:     u.CacheReadTokens + other.CacheReadTokens,
		CacheCreationTokens: u.CacheCreationTokens + other.CacheCreationTokens,
		CostUSD:             u.CostUSD + other.CostUSD,
	}
}

// TurnBudget is the equivalent of QueryEngineConfig.maxTurns / maxBudgetUsd.
// Zero means "no limit", matching the TS default.
type TurnBudget struct {
	MaxTurns   int
	MaxCostUSD float64
}

// ErrInterrupted is returned when Interrupt() was called before/while a turn
// was running. It is the Go-idiomatic version of QueryEngine.abortController
// firing inside submitMessage.
var ErrInterrupted = errors.New("turn engine: interrupted")

// ErrBudgetExceeded is returned when the per-engine budget would be exceeded
// by the next turn. Mirrors the maxTurns / maxBudgetUsd guard in QueryEngine.
var ErrBudgetExceeded = errors.New("turn engine: budget exceeded")

// TurnEngine wraps a ControlSequenceEngine and adds interrupt + budget on top.
// It deliberately does not replace ControlSequenceEngine — it composes it,
// the same way QueryEngine in claude-code-src composes the message stream
// rather than reimplementing tool execution itself.
type TurnEngine struct {
	Sequence *ControlSequenceEngine
	Budget   TurnBudget

	mu          sync.Mutex
	interrupted bool
	turnsRun    int
	totalUsage  TurnUsage
}

func NewTurnEngine(sequence *ControlSequenceEngine, budget TurnBudget) *TurnEngine {
	return &TurnEngine{
		Sequence: sequence,
		Budget:   budget,
	}
}

// Submit runs a single turn through the underlying ControlSequenceEngine,
// honoring interrupt and budget. The estimated usage for the turn is supplied
// by the caller (the runtime layer) — TurnEngine itself does not know how to
// price tokens, exactly like QueryEngine delegates cost to its providers.
func (t *TurnEngine) Submit(input string, estimated TurnUsage) (SequenceRecord, error) {
	t.mu.Lock()
	if t.interrupted {
		t.mu.Unlock()
		return SequenceRecord{}, ErrInterrupted
	}
	if t.Budget.MaxTurns > 0 && t.turnsRun >= t.Budget.MaxTurns {
		t.mu.Unlock()
		return SequenceRecord{}, ErrBudgetExceeded
	}
	projected := t.totalUsage.Add(estimated)
	if t.Budget.MaxCostUSD > 0 && projected.CostUSD > t.Budget.MaxCostUSD {
		t.mu.Unlock()
		return SequenceRecord{}, ErrBudgetExceeded
	}
	t.mu.Unlock()

	record := t.Sequence.Handle(input)

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.interrupted {
		return record, ErrInterrupted
	}
	t.turnsRun++
	t.totalUsage = projected
	return record, nil
}

// Interrupt is the Go equivalent of QueryEngine.interrupt(): it flips a flag
// the next Submit call will observe. It is safe to call from any goroutine.
func (t *TurnEngine) Interrupt() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.interrupted = true
}

// Reset clears the interrupt flag so the engine can be reused after a stop.
func (t *TurnEngine) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.interrupted = false
}

// TotalUsage returns a snapshot of accumulated usage across all turns.
func (t *TurnEngine) TotalUsage() TurnUsage {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totalUsage
}

// TurnsRun returns how many turns have been successfully accounted for.
func (t *TurnEngine) TurnsRun() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.turnsRun
}
