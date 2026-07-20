package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

type confKey struct{}

// confFromCtx retrieves the Conf cached by PersistentPreRunE. If absent (e.g.
// command-specific call), it builds it on demand from cmd.
func confFromCtx(cmd *cobra.Command) (*Conf, error) {
	if v, ok := cmd.Context().Value(confKey{}).(*Conf); ok && v != nil {
		return v, nil
	}
	return NewConf(cmd)
}

func Setup(root *cobra.Command) {
	addConfigurationFlag(root)
	addLoggerFlags(root)
	addGitHubFlags(root)
	addFleetFlags(root)
	addUIFlags(root)
	addFormatFlag(root)
	addAllFlag(root)
	addPlanFlag(root)

	root.AddCommand(versionCmd)

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		conf, err := NewConf(cmd)
		if err != nil {
			return err
		}
		lg := NewLogger(conf)
		slog.SetDefault(lg)
		lg.Debug("starting", "version", Version, "build", Build, "date", BuildDate)

		ctx := context.WithValue(cmd.Context(), confKey{}, conf)

		planPath, _ := cmd.Flags().GetString("plan")
		if planPath != "" {
			plan, err := LoadPlan(planPath)
			if err != nil {
				return fmt.Errorf("plan: %w", err)
			}
			ctx = ctxWithPlan(ctx, plan)

			if !cmd.Flags().Changed("format") && plan.Format != "" {
				sharedFormat = plan.Format
			}
			if !cmd.Flags().Changed("all") {
				sharedAll = plan.All
			}
		}

		cmd.SetContext(ctx)
		return nil
	}
}

func SetupCommands(root *cobra.Command) {
	root.AddCommand(
		newDiscoverCmd(),
		newInitCmd(),
		newSyncCmd(),
		newListCmd(),
		newStatusCmd(),
		newPRsCmd(),
		newRunCmd(),
	)
}
