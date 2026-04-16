package api

const DefaultModel = "qwen3:8b"

type Provider interface {
	SendMessage(request MessageRequest) (MessageResponse, error)
}

type MockProvider struct{}

func (MockProvider) SendMessage(request MessageRequest) (MessageResponse, error) {
	return MessageResponse{Text: "[go provider] model=" + request.Model + " prompt=" + request.Prompt}, nil
}
