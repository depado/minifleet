package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/depado/gorich"
	"github.com/depado/minifleet/internal/fleet"
)

func printBulkSummary(result *fleet.BulkResult, dryRun bool, conf *Conf) {
	if !conf.Console.IsTerminal() {
		return
	}

	summary := fmt.Sprintf("Completed: %d succeeded, %d skipped, %d failed in %s",
		result.Succeeded, result.Skipped, result.Failed, result.Elapsed.Round(time.Millisecond))
	if dryRun {
		gorich.Println("[yellow]would have[/] " + summary)
		return
	}
	conf.PrintInfo(summary)

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
