package fleet

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/depado/gorich/progress"
)

type RepoTask struct {
	RepoName string
	ID       string
	FullName string // owner/repo when known; empty for local-only scans
	Dir      string // local filesystem path (for run/exec); optional
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
	Progress       bool
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

	var p *progress.Progress
	var overallID *progress.TaskID
	var slots []progress.TaskID

	if e.cfg.Progress && e.cfg.ProgressConfig.Description != "" {
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

		slotSection := p.AddSection(
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

		slots = make([]progress.TaskID, workers)
		for i := range slots {
			slots[i] = slotSection.AddTask("[dim]idle[/]", nil)
		}
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
		wg.Add(1)
		go func(slotIdx int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case task, ok := <-taskCh:
					if !ok {
						return
					}

					if p != nil && slotIdx < len(slots) {
						p.Update(slots[slotIdx], progress.TaskUpdateConfig{
							Description: ptr(fmt.Sprintf("[cyan]%s[/]", task.RepoName)),
						})
					}

					taskStart := time.Now()
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

					if p != nil && slotIdx < len(slots) {
						p.ResetTask(slots[slotIdx], false)
						var desc string
						switch status {
						case StatusSuccess:
							desc = fmt.Sprintf("[green]%s[/]", task.RepoName)
						case StatusSkipped:
							desc = fmt.Sprintf("[dim]%s ↷[/]", task.RepoName)
						case StatusFailed:
							desc = fmt.Sprintf("[red]%s[/]", task.RepoName)
						}
						p.Update(slots[slotIdx], progress.TaskUpdateConfig{Description: ptr(desc)})
						if len(taskCh) > 0 {
							time.Sleep(300 * time.Millisecond)
							p.Update(slots[slotIdx], progress.TaskUpdateConfig{Description: ptr("[dim]idle[/]")})
						}
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
		}(i)
	}
	wg.Wait()

	if p != nil {
		for _, tid := range slots {
			p.RemoveTask(tid)
		}
		p.Done(*overallID, fmt.Sprintf("[bold]%s[/]", e.cfg.ProgressConfig.Description))
	}

	result.Succeeded = int(succeeded.Load())
	result.Skipped = int(skipped.Load())
	result.Failed = int(failed.Load())
	result.Elapsed = time.Since(start)
	return result
}

func isSkipError(err error) bool {
	_, ok := err.(*SkipError)
	return ok
}

func ptr[T any](v T) *T { return &v }
