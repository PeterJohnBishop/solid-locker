package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/peterjohnbishop/solid-locker/vault"
)

// model represents the state of our TUI.
type model struct {
	storage *vault.Storage
	files   []string
	err     error
}

// msg to pass the database results back to the update loop.
type filesFetchedMsg struct {
	files []string
	err   error
}

func (m model) Init() tea.Cmd {

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		files, err := m.storage.GetAllFileNames(ctx)
		return filesFetchedMsg{files: files, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case filesFetchedMsg:
		m.err = msg.err
		m.files = msg.files
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	if m.err != nil {
		return tea.NewView(fmt.Sprintf("\n  Error fetching files: %v\n\n  Press 'q' to quit.", m.err))
	}

	if m.files == nil {
		return tea.NewView("\n  Loading files from database...\n")
	}

	if len(m.files) == 0 {
		return tea.NewView("\n  No files stored on the server yet.\n\n  Press 'q' to quit.")
	}

	var b strings.Builder
	b.WriteString("\n  Secure File Vault\n  -----------------\n\n")
	for _, f := range m.files {
		b.WriteString(fmt.Sprintf("  • %s\n", f))
	}
	b.WriteString("\n  Press 'q' to quit.\n")

	return tea.NewView(b.String())
}
