package tui

import (
	"context"
	"fmt"

	// "os"
	"strings"
	"time"

	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/peterjohnbishop/solid-locker/encryption"
	"github.com/peterjohnbishop/solid-locker/vault"
)

var (
	selectedItemStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	focusedHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)

	pathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("212")).
			Padding(1, 2).
			Margin(1, 2)
)

type model struct {
	storage     *vault.Storage
	files       []vault.VaultFile
	err         error
	filepicker  filepicker.Model
	cursor      int
	message     string
	showPicker  bool
	isLocalhost bool
}

type filesFetchedMsg struct {
	files []vault.VaultFile
	err   error
}

type successMsg struct {
	msg string
}

type errorMsg struct {
	err error
}

func initialModel(isLocal bool) model {
	return model{
		isLocalhost: isLocal,
	}
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		files, err := m.storage.GetAllFiles(ctx)
		return filesFetchedMsg{files: files, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		calculatedHeight := msg.Height - 10
		if calculatedHeight < 5 {
			calculatedHeight = 5
		}
		if calculatedHeight > 15 {
			calculatedHeight = 15
		}
		fp := m.filepicker
		fp.SetHeight(calculatedHeight)
		m.filepicker = fp
		return m, nil

	case tea.KeyPressMsg:
		// Global hotkeys
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.showPicker {
				m.showPicker = false
				return m, nil
			}
		case "q":
			if !m.showPicker {
				return m, tea.Quit
			}
		}

		if !m.showPicker {
			switch msg.String() {
			case "u":
				if !m.isLocalhost {
					return m, nil
				}
				m.showPicker = true
				return m, nil

			case "down", "j":
				if m.cursor < len(m.files)-1 {
					m.cursor++
				}
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}

			// case "d":
			// 	if len(m.files) > 0 {
			// 		selectedVaultFile := m.files[m.cursor]
			// 		m.message = fmt.Sprintf("Decrypting %s...", selectedVaultFile.Filename)

			// 		return m, func() tea.Msg {
			// 			ctx := context.Background()
			// 			cwd, _ := os.Getwd()

			// 			err := vault.DownloadLocalFile(ctx, selectedVaultFile.ID, cwd, m.storage, encryption.SaltMaster)
			// 			if err != nil {
			// 				return errorMsg{err}
			// 			}

			// 			return successMsg{fmt.Sprintf("Successfully extracted: %s", selectedVaultFile.Filename)}
			// 		}
			// 	}
			case "d":
				if len(m.files) > 0 {
					selectedVaultFile := m.files[m.cursor]

					// Generate the exact native SSH command
					cmdStr := fmt.Sprintf("ssh -p 23234 localhost get %s > \"%s\"",
						selectedVaultFile.ID,
						selectedVaultFile.Filename,
					)

					m.message = "Download command copied to your local clipboard!"

					// tea.SetClipboard sends the string to the user's laptop clipboard
					return m, tea.SetClipboard(cmdStr)
				}
			}
			return m, nil
		}

	case successMsg:
		m.message = msg.msg
		return m, nil

	case errorMsg:
		m.err = msg.err
		m.message = "An error occurred."
		return m, nil

	case filesFetchedMsg:
		m.err = msg.err
		m.files = msg.files

		if m.message == "Encrypting and vaulting file..." && msg.err == nil {
			m.message = "Upload complete!"
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		m.showPicker = false
		m.message = "Encrypting and vaulting file..."

		return m, func() tea.Msg {
			ctx := context.Background()

			_, err := vault.UploadLocalFile(ctx, path, m.storage, encryption.SaltMaster)
			if err != nil {
				return errorMsg{err}
			}

			updatedFiles, err := m.storage.GetAllFiles(ctx)
			return filesFetchedMsg{files: updatedFiles, err: err}
		}
	}

	return m, cmd
}

func (m model) View() tea.View {
	var b strings.Builder

	// file picker
	if m.showPicker {
		var popup strings.Builder
		popup.WriteString(focusedHeaderStyle.Render("UPLOAD TO VAULT"))
		popup.WriteString("\n\n")

		currentPath := m.filepicker.CurrentDirectory

		popup.WriteString(fmt.Sprintf("Directory: %s\n\n", pathStyle.Render(currentPath)))

		popup.WriteString(m.filepicker.View())
		popup.WriteString("\n\n(up/down: navigate • right: enter folder • enter: select • esc: cancel)")

		b.WriteString(popupStyle.Render(popup.String()))
		return tea.NewView(b.String())
	}

	// file list
	b.WriteString("\n\n  ")
	b.WriteString(focusedHeaderStyle.Render("CURRENT VAULT FILES"))
	b.WriteString("\n  -------------------------------\n")

	if m.message != "" {
		b.WriteString(fmt.Sprintf("  Status: %s\n\n", m.message))
	}

	if m.err != nil {
		b.WriteString(fmt.Sprintf("  Error: %v\n", m.err))
	} else if m.files == nil {
		b.WriteString("  Loading files...\n")
	} else if len(m.files) == 0 {
		b.WriteString("  No files stored on the server yet.\n")
	} else {
		for i, f := range m.files {
			cursor := " "
			rowText := ""

			if m.cursor == i {
				cursor = ">"
				rowText = selectedItemStyle.Render(fmt.Sprintf(" %s • %s", cursor, f.Filename))
			} else {
				rowText = fmt.Sprintf(" %s • %s", cursor, f.Filename)
			}

			b.WriteString(rowText)
			b.WriteString("\n")
		}
	}

	if m.isLocalhost {
		b.WriteString("\n  (u: upload new file • d: extract selected • q: quit)\n")
	} else {
		b.WriteString("\n  (d: extract selected • q: quit)\n")
	}

	return tea.NewView(b.String())
}
