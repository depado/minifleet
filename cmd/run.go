package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/depado/minifleet/internal/fleet"
	"github.com/depado/minifleet/internal/manifest"
	"github.com/depado/minifleet/internal/ui"
)

// runResult is the payload stored on each RepoResult by the run command.
type runResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

func newRunCmd() *cobra.Command {
	var (
		filters    Filters
		summary    bool
		dryRun     bool
		shell      string
		blockLines int
	)

	cmd := &cobra.Command{
		Use:   "run -- <command>",
		Short: "Execute a command in each repository directory",
		Long: `Run a shell command across every repository (or a filtered subset).
Use -- to separate flags from the command.

Examples:
  minifleet run --language go -- "go test ./..."
  minifleet run --group backend -- "make lint"
  minifleet run --summary=false --block-lines 5 -- "make build"
  minifleet run --dry-run -- "rm -f foo.txt"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := confFromCtx(cmd)
			if err != nil {
				return err
			}

			input := strings.Join(args, " ")
			ctx := cmd.Context()
			mf, _ := manifest.Load(manifest.ManifestPath())

			scanDir := conf.Fleet.Base
			flat := false
			if conf.Fleet.Path != "" {
				scanDir = expandPath(conf.Fleet.Path)
				flat = true
			}

			tasks, err := fleet.Scan(scanDir, filters.Target, mf, flat)
			if err != nil {
				return fmt.Errorf("scan repos: %w", err)
			}

			tasksWithName := make([]taskWithName, len(tasks))
			for i, t := range tasks {
				tasksWithName[i] = taskWithName{RepoName: t.RepoName, FullName: t.FullName, ID: t.ID}
			}
			tasksWithName, err = filters.ApplyTasks(tasksWithName, mf)
			if err != nil {
				return err
			}

			filtered := make([]fleet.RepoTask, len(tasksWithName))
			for i, t := range tasksWithName {
				filtered[i] = fleet.RepoTask{RepoName: t.RepoName, ID: t.ID, FullName: t.FullName}
			}

			if len(filtered) == 0 {
				ui.PrintDim("No repositories to run in.")
				return nil
			}

			if dryRun {
				ui.PrintInfo(fmt.Sprintf("would run %q in %d repos via %s", input, len(filtered), shell))
				for _, t := range filtered {
					ui.DefaultPrint(fmt.Sprintf("  [dim]%s[/]  [dim]%s[/]", t.RepoName, t.ID))
				}
				return nil
			}

			useLive := !summary && ui.DefaultConsole.IsTerminal()
			var display *runDisplay
			var liveDisplay *liveLive

			if useLive {
				l, d := startLiveDisplay(blockLines)
				liveDisplay = &liveLive{l}
				display = d
				l.Start(ctx)
			}

			var streamMu sync.Mutex
			executor := fleet.NewExecutor(fleet.ExecutorConfig{
				Concurrency: conf.Fleet.Concurrent,
				Progress:    false,
			})

			result := executor.Run(ctx, filtered, func(ctx context.Context, task fleet.RepoTask) (any, error) {
				start := time.Now()
				c := exec.CommandContext(ctx, shell, "-c", input)
				c.Dir = task.ID
				c.Env = os.Environ()

				if summary {
					var stdout, stderr bytes.Buffer
					c.Stdout = &stdout
					c.Stderr = &stderr
					err := c.Run()
					res := runResult{
						Stdout:   stdout.String(),
						Stderr:   stderr.String(),
						Duration: time.Since(start),
					}
					if err != nil {
						if exitErr, ok := err.(*exec.ExitError); ok {
							res.ExitCode = exitErr.ExitCode()
						} else {
							return res, err
						}
					}
					if res.ExitCode != 0 {
						return res, fmt.Errorf("exit %d", res.ExitCode)
					}
					return res, nil
				}

				if useLive {
					idx := display.startBlock(task.RepoName)
					w := newBlockWriter(idx, display)
					c.Stdout = w
					c.Stderr = w
					err := c.Run()
					elapsed := time.Since(start)
					res := runResult{Duration: elapsed}
					code := 0
					if err != nil {
						if exitErr, ok := err.(*exec.ExitError); ok {
							code = exitErr.ExitCode()
							res.ExitCode = code
						} else {
							display.finish(idx, -1, elapsed)
							return res, err
						}
					}
					display.finish(idx, code, elapsed)
					if code != 0 {
						return res, fmt.Errorf("exit %d", code)
					}
					return res, nil
				}

				// Non-terminal streaming: prefixWriter + inline result line
				pw := newPrefixWriter(task.RepoName, &streamMu)
				c.Stdout = pw
				c.Stderr = pw
				err := c.Run()
				elapsed := time.Since(start)
				res := runResult{Duration: elapsed}
				code := 0
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						code = exitErr.ExitCode()
						res.ExitCode = code
					} else {
						streamMu.Lock()
						ui.DefaultPrint(fmt.Sprintf("[red]✗ %s[/] [dim](%s)[/]", task.RepoName, elapsed.Round(time.Millisecond)))
						streamMu.Unlock()
						return res, err
					}
				}
				streamMu.Lock()
				if code == 0 {
					ui.DefaultPrint(fmt.Sprintf("[green]✓ %s[/] [dim](%s)[/]", task.RepoName, elapsed.Round(time.Millisecond)))
				} else {
					ui.DefaultPrint(fmt.Sprintf("[red]✗ exit %d %s[/] [dim](%s)[/]", code, task.RepoName, elapsed.Round(time.Millisecond)))
				}
				streamMu.Unlock()
				if code != 0 {
					return res, fmt.Errorf("exit %d", code)
				}
				return res, nil
			})

			if liveDisplay != nil {
				liveDisplay.Stop()
			}

			if summary {
				printRunSummary(result)
			}
			return nil
		},
	}

	addFilterFlags(cmd, &filters)
	cmd.Flags().BoolVar(&summary, "summary", true, "one line per repo; --summary=false shows live output blocks")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would run; do not execute")
	cmd.Flags().StringVar(&shell, "shell", "sh", "shell to invoke (default sh)")
	cmd.Flags().IntVar(&blockLines, "block-lines", 3, "output lines per repo block in live mode (--summary=false)")

	return cmd
}

// liveLive wraps live.Live to avoid the import in run.go's type declarations.
type liveLive struct {
	stopper interface{ Stop() }
}

func (l *liveLive) Stop() { l.stopper.Stop() }

func printRunSummary(result *fleet.BulkResult) {
	for _, r := range result.Results {
		res, _ := r.Payload.(runResult)
		var mark string
		switch r.Status {
		case fleet.StatusFailed:
			mark = fmt.Sprintf("[red]✗ exit %d[/]", res.ExitCode)
		case fleet.StatusSkipped:
			mark = "[dim]↷[/]"
		default:
			mark = "[green]✓[/]"
		}
		dur := ""
		if res.Duration > 0 {
			dur = fmt.Sprintf(" [dim](%s)[/]", res.Duration.Round(time.Millisecond))
		}
		ui.DefaultPrint(fmt.Sprintf("%s [bold]%s[/]%s", mark, r.Task.RepoName, dur))

		if r.Status == fleet.StatusFailed {
			if res.Stderr != "" {
				ui.DefaultPrint("[red]  stderr:[/]")
				ui.DefaultPrint(indent(res.Stderr, "    "))
			}
			if res.Stdout != "" {
				ui.DefaultPrint("[dim]  stdout:[/]")
				ui.DefaultPrint(indent(res.Stdout, "    "))
			}
		}
	}

	ui.PrintInfo(fmt.Sprintf("Completed: %d succeeded, %d skipped, %d failed in %s",
		result.Succeeded, result.Skipped, result.Failed, result.Elapsed.Round(time.Millisecond)))
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}

// prefixWriter prefixes each line with repo › under a shared mutex so streaming
// output from concurrent repos stays line-coherent. Used when not in a terminal.
type prefixWriter struct {
	name string
	mu   *sync.Mutex
	buf  []byte
}

func newPrefixWriter(name string, mu *sync.Mutex) *prefixWriter {
	return &prefixWriter{name: name, mu: mu}
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.buf[:i]
		w.buf = w.buf[i+1:]
		w.mu.Lock()
		ui.DefaultPrint(fmt.Sprintf("[dim]%s ›[/] %s", w.name, string(line)))
		w.mu.Unlock()
	}
	return len(p), nil
}