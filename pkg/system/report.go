package system

type StarterSystemReport struct {
	AppName           string
	CommandCount      int
	ToolCount         int
	PluginCount       int
	ServerDescription string
	LSPSummary        string
	RustAnchor        string
	TurnCount         int
	HandledRequestCount int
	LifecycleState    string
	LastRoute         string
	LastPhaseHistory  []string
	LastTurnInput     string
}
