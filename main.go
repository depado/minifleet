package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := &cobra.Command{
		Use:           "minifleet",
		Short:         "Minimal fleet management for your GitHub organization",
		Version:       cmd.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Setup(root)
	cmd.SetupCommands(root)

	if err := root.ExecuteContext(ctx); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
