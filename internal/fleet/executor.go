package fleet

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/depado/gorich/progress"
	"github.com/depado/minifleet/internal/git"
)

type RepoTask struct {
	RepoName string // short repo name (directory name / API name)
	ID       string // stable identity: "owner/repo" when known, else RepoName
	FullName string // "owner/repo" when known; empty for local-only scans
	Dir      string // local filesystem path — the sole path used for git/exec
}

type RepoResult struct {
	Task     RepoTask
	Status   ResultStatus
	Err      error
	Duration time.Duration
	Payload  any
}

type ResultStatus int

const (
	StatusSuccess ResultStatus = iota
	StatusSkipped
	StatusFailed
)

func (s ResultStatus) String() string {
	switch s {
	case StatusSkipped:
		return "skipped"
	case StatusFailed:
		return "failed"
	default:
		return "ok"
	}
}

type BulkResult struct {
	Total     int
	Succeeded int
	Skipped   int
	Failed    int
	Results   []RepoResult
	Elapsed   time.Duration
}

type ExecutorConfig struct {
	Concurrency    int
	Interactive    bool
	ProgressConfig ProgressConfig
}

type ProgressConfig struct {
	Description string
}

type SkipError struct{ Reason string }

func (e *SkipError) Error() string { return e.Reason }

type Operation func(ctx context.Context, task RepoTask) (any, error)

type Executor struct {
	cfg ExecutorConfig
}

func NewExecutor(cfg ExecutorConfig) *Executor {
	return &Executor{cfg: cfg}
}

func (e *Executor) Run(ctx context.Context, tasks []RepoTask, op Operation) *BulkResult {
	result := &BulkResult{
		Total:   len(tasks),
		Results: make([]RepoResult, 0, len(tasks)),
	}
	start := time.Now()

	workers := e.cfg.Concurrency
	if workers <= 0 {
		workers = 1
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}

	slog.Debug("executor starting", "tasks", len(tasks), "workers", workers)

	var p *progress.Progress
	var overallID *progress.TaskID
	var slotSection *progress.Section

	if e.cfg.Interactive && e.cfg.ProgressConfig.Description != "" {
		p = progress.New(
			progress.WithColumns(
				progress.NewSpinnerColumn(
					progress.WithSpinnerName("dots"),
				),
				progress.DescriptionColumn(),
				progress.NewBarColumn(progress.WithBarWidth(30)),
				progress.NewMofNCompleteColumn("/"),
				progress.NewTimeRemainingColumn(),
			),
			progress.WithRefreshRate(10),
		)

		slotSection = p.AddSection(
			progress.WithSectionColumns(
				progress.NewSpinnerColumn(
					progress.WithSpinnerName("dots"),
				),
				progress.DescriptionColumn(),
			),
		)

		p.Start(ctx)
		defer p.Stop()

		total := float64(len(tasks))
		tid := p.AddTask(fmt.Sprintf("[bold]%s[/]", e.cfg.ProgressConfig.Description), &total)
		overallID = &tid
	}

	taskCh := make(chan RepoTask, len(tasks))
	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded atomic.Int32
		skipped   atomic.Int32
		failed    atomic.Int32
	)

	for i := 0; i < workers; i++ {
		wg.Go(func() {
			var slotID progress.TaskID
			hasSlot := false
			for {
				select {
				case <-ctx.Done():
					if hasSlot {
						p.RemoveTask(slotID)
					}
					return
				case task, ok := <-taskCh:
					if !ok {
						if hasSlot {
							p.RemoveTask(slotID)
						}
						return
					}

					if hasSlot {
						p.RemoveTask(slotID)
						hasSlot = false
					}

					if p != nil {
						slotID = slotSection.AddTask(fmt.Sprintf("[cyan]%s[/]", task.RepoName), nil)
						hasSlot = true
					}

					taskStart := time.Now()
					if !e.cfg.Interactive {
						slog.Debug("processing", "repo", task.RepoName, "id", task.ID)
					}
					payload, err := op(ctx, task)
					dur := time.Since(taskStart)

					status := StatusSuccess
					if err != nil {
						if isSkipError(err) {
							status = StatusSkipped
							skipped.Add(1)
						} else {
							status = StatusFailed
							failed.Add(1)
						}
					} else {
						succeeded.Add(1)
					}

					attrs := []any{"repo", task.RepoName, "id", task.ID, "status", status.String(), "duration", dur}
					if err != nil {
						attrs = append(attrs, "error", err)
					}
					if !e.cfg.Interactive {
						slog.Debug("processed", attrs...)
					}

					if p != nil {
						var desc string
						switch status {
						case StatusSuccess:
							desc = fmt.Sprintf("[green]%s[/]", task.RepoName)
						case StatusSkipped:
							desc = fmt.Sprintf("[dim]%s ↷[/]", task.RepoName)
						case StatusFailed:
							desc = fmt.Sprintf("[red]%s[/]", task.RepoName)
						}
						p.Update(slotID, progress.TaskUpdateConfig{Description: &desc})
						if len(taskCh) > 0 {
							time.Sleep(300 * time.Millisecond)
						}
						p.RemoveTask(slotID)
						hasSlot = false
					}

					if p != nil && overallID != nil {
						p.Advance(*overallID, 1)
					}

					mu.Lock()
					result.Results = append(result.Results, RepoResult{
						Task:     task,
						Status:   status,
						Err:      err,
						Duration: dur,
						Payload:  payload,
					})
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()

	if p != nil {
		p.Done(*overallID, fmt.Sprintf("[bold]%s[/]", e.cfg.ProgressConfig.Description))
	}

	result.Succeeded = int(succeeded.Load())
	result.Skipped = int(skipped.Load())
	result.Failed = int(failed.Load())
	result.Elapsed = time.Since(start)
	slog.Debug("executor finished",
		"tasks", len(tasks),
		"succeeded", result.Succeeded,
		"skipped", result.Skipped,
		"failed", result.Failed,
		"elapsed", result.Elapsed.Round(time.Millisecond),
	)
	return result
}

func isSkipError(err error) bool {
	if _, ok := err.(*SkipError); ok {
		return true
	}
	// git.SkipError mirrors SkipError so the git package can signal
	// "skip this repo" without importing fleet (would be a cycle).
	if _, ok := err.(*git.SkipError); ok {
		return true
	}
	return false
}
