package system

type StarterSystemReport struct {
	AppName             string
	CommandCount        int
	ToolCount           int
	PluginCount         int
	ServerDescription   string
	LSPSummary          string
	RuntimeAnchor       string
	TurnCount           int
	HandledRequestCount int
	LifecycleState      string
	LastRoute           string
	LastPhaseHistory    []string
	LastTurnInput       string
}
