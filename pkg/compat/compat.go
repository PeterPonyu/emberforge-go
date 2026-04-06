package compat

type UpstreamPaths struct {
	ClaudeCommandsTS string
	ClaudeToolsTS    string
	EmberRuntimeLib  string
}

func DefaultUpstreamPaths() UpstreamPaths {
	return UpstreamPaths{
		ClaudeCommandsTS: "/home/zeyufu/Desktop/claude-code-src/commands.ts",
		ClaudeToolsTS:    "/home/zeyufu/Desktop/claude-code-src/tools.ts",
		EmberRuntimeLib:  "/home/zeyufu/Desktop/emberforge/crates/runtime/src/lib.rs",
	}
}
