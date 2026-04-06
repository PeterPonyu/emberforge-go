package system

type LifecycleState string

const (
	LifecycleCreated      LifecycleState = "created"
	LifecycleBootstrapping LifecycleState = "bootstrapping"
	LifecycleReady        LifecycleState = "ready"
	LifecycleDispatching  LifecycleState = "dispatching"
	LifecycleExecuting    LifecycleState = "executing"
	LifecyclePersisting   LifecycleState = "persisting"
	LifecycleReporting    LifecycleState = "reporting"
	LifecycleShuttingDown LifecycleState = "shutting_down"
	LifecycleStopped      LifecycleState = "stopped"
)

type LifecycleTracker struct {
	current LifecycleState
	history []LifecycleState
}

func NewLifecycleTracker() *LifecycleTracker {
	return &LifecycleTracker{
		current: LifecycleCreated,
		history: []LifecycleState{LifecycleCreated},
	}
}

func (l *LifecycleTracker) Transition(next LifecycleState) {
	l.current = next
	l.history = append(l.history, next)
}

func (l *LifecycleTracker) Current() LifecycleState {
	return l.current
}

func (l *LifecycleTracker) History() []LifecycleState {
	copied := make([]LifecycleState, len(l.history))
	copy(copied, l.history)
	return copied
}
