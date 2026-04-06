package lsp

type Manager struct{}

func (Manager) Summary() string {
	return "Go LSP manager (pkg/lsp)"
}
