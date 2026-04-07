package api

const DefaultModel = "claude-sonnet-4-6"

type Provider interface {
	SendMessage(request MessageRequest) (MessageResponse, error)
}

type MockProvider struct{}

func (MockProvider) SendMessage(request MessageRequest) (MessageResponse, error) {
	return MessageResponse{Text: "[go provider] model=" + request.Model + " prompt=" + request.Prompt}, nil
}
