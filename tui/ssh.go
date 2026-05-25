package tui

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	bm "charm.land/wish/v2/bubbletea"
	lm "charm.land/wish/v2/logging"
	"github.com/charmbracelet/ssh"
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
			bm.Middleware(teaHandler),
			lm.Middleware(),
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
