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

		planPath, _ := cmd.Flags().GetString("plan")
		var plan *Plan
		if planPath != "" {
			plan, err = LoadPlan(planPath)
			if err != nil {
				return fmt.Errorf("plan: %w", err)
			}
			if plan.Interactive != "" && plan.Interactive != conf.Interactive {
				conf.Interactive = plan.Interactive
				conf.Console = newConsole(conf.Interactive)
			}
			if !cmd.Flags().Changed("json") && plan.JSON {
				sharedJSON = plan.JSON
			}
			if !cmd.Flags().Changed("all") {
				sharedAll = plan.All
			}
		}

		if sharedJSON {
			conf.Interactive = "never"
			conf.Console = newConsole("never")
		}

		lg := NewLogger(conf)
		slog.SetDefault(lg)
		lg.Debug("starting", "version", Version, "build", Build, "date", BuildDate)

		ctx := context.WithValue(cmd.Context(), confKey{}, conf)
		if plan != nil {
			ctx = ctxWithPlan(ctx, plan)
		}
		cmd.SetContext(ctx)
		return nil
	}
}

func SetupCommands(root *cobra.Command) {
	root.AddCommand(
		newDiscoverCmd(),
		newFetchCmd(),
		newInitCmd(),
		newSyncCmd(),
		newListCmd(),
		newStatusCmd(),
		newPRsCmd(),
		newRunCmd(),
	)
}
