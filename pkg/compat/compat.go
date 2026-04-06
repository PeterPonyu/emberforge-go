package compat

type UpstreamPaths struct {
	ClaudeCommandsTS string
	ClaudeToolsTS    string
	EmberRuntimeLib  string
}

func DefaultUpstreamPaths() UpstreamPaths {
	return UpstreamPaths{
		ClaudeCommandsTS: "",
		ClaudeToolsTS:    "",
		EmberRuntimeLib:  "github.com/PeterPonyu/emberforge-go/pkg/runtime",
	}
}
