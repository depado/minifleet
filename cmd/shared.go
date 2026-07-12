package cmd

import (
	"fmt"
	"time"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/ui"
)

func printBulkSummary(result *fleet.BulkResult, dryRun bool) {
	prefix := ""
	if dryRun {
		prefix = "[yellow]would have[/] "
	}
	ui.PrintInfo(fmt.Sprintf("%sCompleted: %d succeeded, %d skipped, %d failed in %s",
		prefix, result.Succeeded, result.Skipped, result.Failed, result.Elapsed.Round(time.Millisecond)))

	for _, r := range result.Results {
		if r.Status != fleet.StatusFailed {
			continue
		}
		ui.PrintError(fmt.Sprintf("%s: %v", r.Task.RepoName, r.Err))
	}
}
