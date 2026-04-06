package api

const DefaultModel = "claude-sonnet-4-6"

type Provider interface {
	SendMessage(request MessageRequest) MessageResponse
}

type MockProvider struct{}

func (MockProvider) SendMessage(request MessageRequest) MessageResponse {
	return MessageResponse{Text: "[go provider] model=" + request.Model + " prompt=" + request.Prompt}
}
