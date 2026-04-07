package system

type StarterSystemConfig struct {
	AppName         string
	Port            int
	CommandDemoName string
	Greeting        string
	ToolDemoCommand string
	MaxTurns        int
	MaxCostUSD      float64
}

func DefaultStarterSystemConfig() StarterSystemConfig {
	return StarterSystemConfig{
		AppName:         "emberforge-go system",
		Port:            8080,
		CommandDemoName: "help",
		Greeting:        "hello from go system",
		ToolDemoCommand: "printf hello",
		MaxTurns:        16,
		MaxCostUSD:      1.0,
	}
}
