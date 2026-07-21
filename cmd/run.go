package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/depado/gorich/live"
	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/git"
	"github.com/depado/minifleet/internal/ui"
)

// runLine is a single output line captured during a run, tagged with its source.
type runLine struct {
	Stream string `json:"stream"` // "stdout" or "stderr"
	Text   string `json:"text"`
}

// runResult is the payload stored on each RepoResult by the run command.
type runResult struct {
	ExitCode int           `json:"exit_code"`
	Lines    []runLine     `json:"lines,omitempty"`
	Duration time.Duration `json:"duration"`
}

// runMode determines how output is displayed.
type runMode int

const (
	modeAuto    runMode = iota // TTY → live blocks, non-TTY → summary
	modeLive                   // force live block display
	modeSummary                // force summary (capture + print per repo)
)

func newRunCmd() *cobra.Command {
	var (
		filters     Filters
		summary     bool
		progress    bool
		summarySet  bool
		progressSet bool
		dryRun      bool
		shell       string
		blockLines  int
	)

	cmd := &cobra.Command{
		Use:   "run -- <command>",
		Short: "Execute a command in each repository directory",
		Long: `Run a shell command across every repository (or a filtered subset).
Use -- to separate flags from the command.

By default, live block output is used in a terminal (animated spinners,
stdout dim, stderr red) and a summary is printed when piped. Override with
--summary or --progress. Use --format json for machine-readable output.

Examples:
  minifleet run --language go -- "go test ./..."
  minifleet run --group backend -- "make lint"
  minifleet run --progress --block-lines 5 -- "make build"
  minifleet run --summary -- "git branch --show-current"
  minifleet run --format json -- "make test"
  minifleet run --dry-run -- "rm -f foo.txt"
  minifleet run --plan plan.yml`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			plan := planFromCtx(ctx)
			ApplyPlan(&filters, plan, cmd)

			if plan != nil {
				if !cmd.Flags().Changed("shell") && plan.Shell != "" {
					shell = plan.Shell
				}
				if !cmd.Flags().Changed("dry-run") {
					dryRun = plan.DryRun
				}
				if !cmd.Flags().Changed("summary") {
					summary = plan.Summary
				}
				if !cmd.Flags().Changed("progress") {
					progress = plan.Progress
				}
				if !cmd.Flags().Changed("block-lines") && plan.BlockLines > 0 {
					blockLines = plan.BlockLines
				}
			}

			input := strings.Join(args, " ")
			if input == "" {
				if plan != nil && plan.Command != "" {
					input = plan.Command
				} else {
					return fmt.Errorf("no command specified; use --plan with a command field, or pass args after --")
				}
			}

			targets, err := planTargets(conf, plan, sharedAll)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				ui.PrintDim("No fleet in the current directory and no known fleets. Run 'minifleet discover <owner>' first.")
				return nil
			}

			type fleetTasks struct {
				target fleetTarget
				tasks  []fleet.RepoTask
			}
			var planned []fleetTasks
			totalCount := 0
			for _, t := range targets {
				tasks, err := reposForTarget(ctx, t, filters)
				if err != nil {
					return fmt.Errorf("scan %s: %w", t.Dir, err)
				}
				if len(tasks) == 0 {
					continue
				}
				planned = append(planned, fleetTasks{target: t, tasks: tasks})
				totalCount += len(tasks)
			}

			if totalCount == 0 {
				ui.PrintDim("No repositories to run in.")
				return nil
			}

			if dryRun {
				ui.PrintInfo(fmt.Sprintf("would run %q in %d repos via %s", input, totalCount, shell))
				for _, p := range planned {
					if len(planned) > 1 {
						ui.DefaultPrint(fmt.Sprintf("[bold]%s[/] [dim](%s)[/]", p.target.Owner, p.target.Dir))
					}
					for _, t := range p.tasks {
						ui.DefaultPrint(fmt.Sprintf("  [dim]%s[/]  [dim]%s[/]", t.RepoName, t.Dir))
					}
				}
				return nil
			}

			jsonMode := sharedFormat == "json"

			// Determine display mode.
			// --progress forces live, --summary forces summary.
			// Default: live in TTY, summary when piped. JSON always wins.
			mode := modeAuto
			if progressSet && progress {
				mode = modeLive
			} else if summarySet && summary {
				mode = modeSummary
			}
			useLive := false
			if !jsonMode {
				switch mode {
				case modeLive:
					useLive = true
				case modeSummary:
					useLive = false
				default:
					useLive = ui.DefaultConsole.IsTerminal()
				}
			}

			var display *live.BlockDisplay
			var liveDisplay *live.Live

			if useLive {
				display = live.NewBlockDisplay(
					live.WithBlockMaxLines(blockLines),
					live.WithBlockSpinnerName("dots"),
				)
				liveDisplay = live.New(
					ui.DefaultConsole,
					display,
					live.WithAutoRefresh(true),
					live.WithRefreshRate(15),
				)
				liveDisplay.Start(ctx)
			}

			executor := fleet.NewExecutor(fleet.ExecutorConfig{
				Concurrency: conf.Concurrent,
				Progress:    false,
			})

			var globalResult fleet.BulkResult
			for _, p := range planned {
				if len(planned) > 1 && !useLive && !jsonMode {
					ui.DefaultPrint(fmt.Sprintf("[bold]%s[/] [dim](%s)[/]", p.target.Owner, p.target.Dir))
				}
				result := executor.Run(ctx, p.tasks, func(ctx context.Context, task fleet.RepoTask) (any, error) {
					if !git.IsRepo(ctx, task.Dir) {
						return nil, &fleet.SkipError{Reason: "not cloned"}
					}
					return runOneRepo(ctx, task, input, shell, useLive, jsonMode, display)
				})
				if !useLive && !jsonMode {
					printRunSummary(result)
				}
				globalResult.Succeeded += result.Succeeded
				globalResult.Skipped += result.Skipped
				globalResult.Failed += result.Failed
				globalResult.Results = append(globalResult.Results, result.Results...)
				globalResult.Elapsed += result.Elapsed
			}

			if liveDisplay != nil {
				time.Sleep(200 * time.Millisecond)
				liveDisplay.Stop()
			}

			if jsonMode {
				return outputRunJSON(&globalResult)
			}

			ui.PrintInfo(fmt.Sprintf("Completed: %d succeeded, %d skipped, %d failed in %s",
				globalResult.Succeeded, globalResult.Skipped, globalResult.Failed, globalResult.Elapsed.Round(time.Millisecond)))
			return nil
		},
	}

	addLocalFilterFlags(cmd, &filters)
	cmd.Flags().BoolVar(&summary, "summary", false, "force summary mode (one block per repo with captured output)")
	cmd.Flags().BoolVar(&progress, "progress", false, "force live block mode (animated spinners + streaming output)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would run; do not execute")
	cmd.Flags().StringVar(&shell, "shell", "sh", "shell to invoke (default sh)")
	cmd.Flags().IntVar(&blockLines, "block-lines", 3, "output lines per repo block in live mode")

	// Track whether flags were explicitly set by the user.
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		summarySet = cmd.Flags().Changed("summary")
		progressSet = cmd.Flags().Changed("progress")
		return nil
	}

	return cmd
}

// runOneRepo executes the shell command in one repo's directory and records
// the result according to the chosen output mode (live / summary / json).
func runOneRepo(ctx context.Context, task fleet.RepoTask, input, shell string, useLive, jsonMode bool, display *live.BlockDisplay) (any, error) {
	start := time.Now()
	c := exec.CommandContext(ctx, shell, "-c", input)
	c.Dir = task.Dir
	c.Env = os.Environ()

	res := &runResult{Duration: time.Since(start)}

	// Shared ordering mutex so stdout and stderr lines are captured in
	// arrival order, regardless of which pipe the OS schedules first.
	var orderMu sync.Mutex

	if useLive {
		idx := display.Start(task.RepoName)
		outW := &styledLineWriter{stream: "stdout", result: res, display: display, blockIdx: idx, orderMu: &orderMu}
		errW := &styledLineWriter{stream: "stderr", result: res, display: display, blockIdx: idx, orderMu: &orderMu}
		c.Stdout = outW
		c.Stderr = errW
		err := c.Run()
		elapsed := time.Since(start)
		res.Duration = elapsed
		code := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
				res.ExitCode = code
			} else {
				display.Finish(idx, 1)
				return res, err
			}
		}
		display.Finish(idx, code)
		if code != 0 {
			return res, fmt.Errorf("exit %d", code)
		}
		return res, nil
	}

	// Summary + JSON mode: use styledLineWriter (no display) so both streams
	// are interleaved in arrival order under the shared orderMu.
	outW := &styledLineWriter{stream: "stdout", result: res, orderMu: &orderMu}
	errW := &styledLineWriter{stream: "stderr", result: res, orderMu: &orderMu}
	c.Stdout = outW
	c.Stderr = errW
	err := c.Run()
	res.Duration = time.Since(start)
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
			res.ExitCode = code
		} else {
			return res, err
		}
	}
	if code != 0 {
		return res, fmt.Errorf("exit %d", code)
	}
	return res, nil
}

// jsonRunResult is the JSON representation of a single repo's run result.
type jsonRunResult struct {
	Repo     string    `json:"repo"`
	Owner    string    `json:"owner,omitempty"`
	ExitCode int       `json:"exit_code"`
	Duration string    `json:"duration"`
	Lines    []runLine `json:"lines,omitempty"`
	Error    string    `json:"error,omitempty"`
}

func outputRunJSON(result *fleet.BulkResult) error {
	out := make([]jsonRunResult, 0, len(result.Results))
	for i := range result.Results {
		r := &result.Results[i]
		res, _ := r.Payload.(*runResult)
		jr := jsonRunResult{
			Repo:     r.Task.RepoName,
			ExitCode: -1,
		}
		if res != nil {
			jr.ExitCode = res.ExitCode
			jr.Duration = res.Duration.Round(time.Millisecond).String()
			jr.Lines = res.Lines
		}
		if r.Status == fleet.StatusFailed && r.Err != nil {
			jr.Error = r.Err.Error()
		}
		if r.Task.FullName != "" {
			if parts := strings.SplitN(r.Task.FullName, "/", 2); len(parts) == 2 {
				jr.Owner = parts[0]
			}
		}
		out = append(out, jr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Repo < out[j].Repo })

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

func printRunSummary(result *fleet.BulkResult) {
	for i := range result.Results {
		r := &result.Results[i]
		res, _ := r.Payload.(*runResult)
		dur := ""
		if res != nil && res.Duration > 0 {
			dur = fmt.Sprintf(" [dim](%s)[/]", res.Duration.Round(time.Millisecond))
		}

		var line string
		switch r.Status {
		case fleet.StatusFailed:
			exitCode := -1
			if res != nil {
				exitCode = res.ExitCode
			}
			line = fmt.Sprintf("[red]%s (exit %d)[/]%s", r.Task.RepoName, exitCode, dur)
		case fleet.StatusSkipped:
			line = fmt.Sprintf("[dim]%s ↷[/]%s", r.Task.RepoName, dur)
		default:
			line = fmt.Sprintf("[green]%s[/]%s", r.Task.RepoName, dur)
		}
		ui.DefaultPrint(line)

		// Print captured lines: stdout dim, stderr red
		if res != nil {
			for _, l := range res.Lines {
				if l.Stream == "stderr" {
					ui.DefaultPrint(fmt.Sprintf("  [red]%s[/]", l.Text))
				} else {
					ui.DefaultPrint(fmt.Sprintf("  [dim]%s[/]", l.Text))
				}
			}
		}
	}
}
