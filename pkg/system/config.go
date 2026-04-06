package system

type StarterSystemConfig struct {
	AppName         string
	Port            int
	Greeting        string
	ToolDemoCommand string
}

func DefaultStarterSystemConfig() StarterSystemConfig {
	return StarterSystemConfig{
		AppName:         "emberforge-go system",
		Port:            8080,
		Greeting:        "hello from go system",
		ToolDemoCommand: "printf translated",
	}
}
