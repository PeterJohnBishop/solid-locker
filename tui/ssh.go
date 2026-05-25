package tui

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	bm "charm.land/wish/v2/bubbletea"
	lm "charm.land/wish/v2/logging"
	"github.com/charmbracelet/ssh"
	"github.com/peterjohnbishop/solid-locker/encryption"
	"github.com/peterjohnbishop/solid-locker/vault"
)

func StartSSHServer(storage *vault.Storage) {
	teaHandler := func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		fp := filepicker.New()
		fp.CurrentDirectory, _ = os.Getwd()
		fp.SetHeight(7)

		m := model{
			storage:    storage,
			filepicker: fp,
		}
		return m, nil
	}

	srv, err := wish.NewServer(
		wish.WithAddress("0.0.0.0:23234"),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(
			bm.Middleware(teaHandler),   // runs last
			DownloadMiddleware(storage), //
			lm.Middleware(),             // runs first
		),
	)
	if err != nil {
		log.Fatalf("Could not create SSH server: %v", err)
	}

	go func() {
		log.Println("Starting TUI SSH server on port 23234...")
		if err := srv.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			log.Fatalf("SSH server failed: %v", err)
		}
	}()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-done

	log.Println("Stopping SSH server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Could not stop SSH server gracefully: %v", err)
	}
}

// intercepts SSH commands to stream files directly to the client
func DownloadMiddleware(storage *vault.Storage) wish.Middleware {

	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			cmd := s.Command()
			pty, _, isPty := s.Pty()

			log.Printf("=== NEW SSH CONNECTION ===")
			log.Printf("Is PTY Requested: %v", isPty)
			if isPty {
				log.Printf("PTY Type: %s", pty.Term)
			}
			log.Printf("Raw Command Slice: %#v", cmd)

			if len(cmd) > 0 {
				fullCmd := strings.TrimSpace(strings.Join(cmd, " "))
				fullCmd = strings.TrimPrefix(fullCmd, "sh -c ")
				fullCmd = strings.TrimPrefix(fullCmd, "bash -c ")
				fullCmd = strings.ReplaceAll(fullCmd, "\"", "")
				fullCmd = strings.ReplaceAll(fullCmd, "'", "")

				parts := strings.Fields(fullCmd)
				log.Printf("Parsed Command Parts: %#v", parts)

				if len(parts) >= 2 && parts[0] == "get" {
					fileID := parts[1]
					log.Printf("Attempting to stream file ID: %s", fileID)

					err := storage.StreamRetrieveAndDecrypt(s.Context(), fileID, s, encryption.SaltMaster)
					if err != nil {
						log.Printf("Stream Error: %v", err)
						wish.Fatalln(s, fmt.Sprintf("Failed to download file: %v", err))
						return
					}

					log.Printf("Stream successful!")
					s.Exit(0)
					return
				}
			}

			log.Printf("No valid 'get' command found. Passing to Bubble Tea...")
			next(s)
		}
	}
}
