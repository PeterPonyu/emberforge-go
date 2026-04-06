package system

import (
	"fmt"

	"github.com/PeterPonyu/emberforge-go/pkg/commands"
	"github.com/PeterPonyu/emberforge-go/pkg/runtime"
	"github.com/PeterPonyu/emberforge-go/pkg/telemetry"
)

type SequenceRecord struct {
	RequestID string
	Input     string
	Route     DispatchRoute
	Phases    []LifecycleState
	Output    string
}

type ControlSequenceEngine struct {
	Runtime    *runtime.ConversationRuntime
	Dispatcher SystemDispatcher
	Lifecycle  *LifecycleTracker
	Telemetry  telemetry.TelemetrySink
	RecordsLog []SequenceRecord
	nextID     int
}

func NewControlSequenceEngine(runtimeCore *runtime.ConversationRuntime, dispatcher SystemDispatcher, lifecycle *LifecycleTracker, telemetrySink telemetry.TelemetrySink) *ControlSequenceEngine {
	return &ControlSequenceEngine{
		Runtime:    runtimeCore,
		Dispatcher: dispatcher,
		Lifecycle:  lifecycle,
		Telemetry:  telemetrySink,
		RecordsLog: []SequenceRecord{},
		nextID:     1,
	}
}

func (e *ControlSequenceEngine) Bootstrap() {
	if e.Lifecycle.Current() != LifecycleCreated {
		return
	}
	e.Lifecycle.Transition(LifecycleBootstrapping)
	e.Telemetry.Record(telemetry.Event{Name: "bootstrap_completed", Details: "system ready"})
	e.Lifecycle.Transition(LifecycleReady)
}

func (e *ControlSequenceEngine) Handle(input string) SequenceRecord {
	if e.Lifecycle.Current() == LifecycleCreated {
		e.Bootstrap()
	}

	ctx := ControlSequenceContext{
		RequestID: fmt.Sprintf("req-%d", e.nextID),
		Input:     input,
	}
	e.nextID++

	phases := make([]LifecycleState, 0, 4)
	mark := func(state LifecycleState) {
		e.Lifecycle.Transition(state)
		phases = append(phases, state)
	}

	mark(LifecycleDispatching)
	decision := e.Dispatcher.Dispatch(input)
	ctx.Route = string(decision.Route)

	mark(LifecycleExecuting)
	output := e.executeDecision(decision)

	mark(LifecyclePersisting)
	record := SequenceRecord{RequestID: ctx.RequestID, Input: ctx.Input, Route: decision.Route, Phases: phases, Output: output}
	e.RecordsLog = append(e.RecordsLog, record)
	e.Telemetry.Record(telemetry.Event{Name: "sequence_persisted", Details: fmt.Sprintf("%s:%s", record.RequestID, record.Route)})

	mark(LifecycleReporting)
	e.Telemetry.Record(telemetry.Event{Name: "sequence_reported", Details: output})

	e.Lifecycle.Transition(LifecycleReady)
	return record
}

func (e *ControlSequenceEngine) Shutdown() {
	if e.Lifecycle.Current() == LifecycleStopped {
		return
	}
	e.Lifecycle.Transition(LifecycleShuttingDown)
	e.Telemetry.Record(telemetry.Event{Name: "shutdown_completed", Details: fmt.Sprintf("handled=%d", len(e.RecordsLog))})
	e.Lifecycle.Transition(LifecycleStopped)
}

func (e *ControlSequenceEngine) LastRecord() (SequenceRecord, bool) {
	if len(e.RecordsLog) == 0 {
		return SequenceRecord{}, false
	}
	return e.RecordsLog[len(e.RecordsLog)-1], true
}

func (e *ControlSequenceEngine) executeDecision(decision DispatchDecision) string {
	switch decision.Route {
	case RouteCommand:
		return e.renderCommandOutput(decision.CommandName)
	case RouteTool:
		return e.Runtime.RunTurn("/tool " + decision.Payload)
	case RoutePrompt:
		return e.Runtime.RunTurn(decision.Payload)
	default:
		return "[sequence] unreachable"
	}
}

func (e *ControlSequenceEngine) renderCommandOutput(commandName string) string {
	if commandName == "status" {
		return fmt.Sprintf("[command] status: lifecycle=%s handled=%d", e.Lifecycle.Current(), len(e.RecordsLog))
	}
	if commandName == "model" {
		return "[command] model: registry-driven control sequence starter"
	}
	if command, ok := e.Dispatcher.Commands.Find(commandName); ok {
		return fmt.Sprintf("[command] %s: %s", command.Name, command.Description)
	}
	return fmt.Sprintf("[command] unknown: %s", commandName)
}

func PhaseStrings(phases []LifecycleState) []string {
	result := make([]string, 0, len(phases))
	for _, phase := range phases {
		result = append(result, string(phase))
	}
	return result
}

func DefaultCommandCatalog() []commands.CommandSpec {
	return commands.GetCommands()
}
