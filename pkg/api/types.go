package api

type MessageRequest struct {
	Model  string
	Prompt string
}

type MessageResponse struct {
	Text string
}
