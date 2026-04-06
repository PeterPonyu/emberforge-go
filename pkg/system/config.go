package system

type StarterSystemConfig struct {
	AppName         string
	Port            int
	CommandDemoName string
	Greeting        string
	ToolDemoCommand string
}

func DefaultStarterSystemConfig() StarterSystemConfig {
	return StarterSystemConfig{
		AppName:         "emberforge-go system",
		Port:            8080,
		CommandDemoName: "help",
		Greeting:        "hello from go system",
		ToolDemoCommand: "printf hello",
	}
}
