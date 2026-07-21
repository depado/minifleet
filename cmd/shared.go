package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/depado/gorich"
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
		switch r.Status {
		case fleet.StatusFailed:
			printResultLine("[red]✗ %s[/]", r.Task.RepoName, r.Err)
		case fleet.StatusSkipped:
			if r.Err != nil {
				printResultLine("[yellow]↷ %s[/]", r.Task.RepoName, r.Err)
			}
		}
	}
}

func printResultLine(glyphFmt, repoName string, err error) {
	msg := err.Error()
	if strings.Contains(msg, "\n") {
		gorich.Printf(glyphFmt+"\n", repoName)
		for line := range strings.SplitSeq(msg, "\n") {
			if line != "" {
				gorich.Printf("  [dim]%s[/]\n", line)
			}
		}
	} else {
		gorich.Printf(glyphFmt+" [dim]%s[/]\n", repoName, msg)
	}
}
