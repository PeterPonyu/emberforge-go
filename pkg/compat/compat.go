package compat

type UpstreamPaths struct {
	EmberRuntimeLib string
}

func DefaultUpstreamPaths() UpstreamPaths {
	return UpstreamPaths{
		EmberRuntimeLib: "github.com/PeterPonyu/emberforge-go/pkg/runtime",
	}
}
