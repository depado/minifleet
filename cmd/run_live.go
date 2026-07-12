package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/depado/gorich/console"
	"github.com/depado/gorich/live"
	"github.com/depado/gorich/segment"
	"github.com/depado/gorich/style"

	"github.com/depado/minifleet/internal/ui"
)

type blockStatus int

const (
	blockRunning blockStatus = iota
	blockDone
	blockFailed
)

// runBlock is a single repo's display block: header line + ring buffer of the
// last maxLines output lines. Output lines stay visible after the block finishes.
type runBlock struct {
	repo     string
	status   blockStatus
	lines    []string
	maxLines int
	elapsed  time.Duration
	exitCode int
}

// runDisplay is a console.Renderable that shows a growing list of repo blocks.
// Each block has a header (status + repo name) followed by its last N output lines.
// Old blocks scroll off the top when the display exceeds terminal height.
type runDisplay struct {
	mu       sync.Mutex
	blocks   []runBlock
	maxLines int
}

func newRunDisplay(maxLines int) *runDisplay {
	return &runDisplay{
		maxLines: maxLines,
	}
}

// startBlock appends a new block for the given repo and returns its index.
func (d *runDisplay) startBlock(repo string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.blocks = append(d.blocks, runBlock{
		repo:     repo,
		status:   blockRunning,
		lines:    make([]string, 0, d.maxLines),
		maxLines: d.maxLines,
	})
	return len(d.blocks) - 1
}

func (d *runDisplay) addLine(idx int, line string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	b := &d.blocks[idx]
	b.lines = append(b.lines, line)
	if len(b.lines) > b.maxLines {
		b.lines = b.lines[len(b.lines)-b.maxLines:]
	}
}

func (d *runDisplay) finish(idx int, exitCode int, elapsed time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	b := &d.blocks[idx]
	b.elapsed = elapsed
	b.exitCode = exitCode
	if exitCode == 0 {
		b.status = blockDone
	} else {
		b.status = blockFailed
	}
}

// Render implements console.Renderable.
func (d *runDisplay) Render(c *console.Console, opts console.Options) []segment.Segment {
	d.mu.Lock()
	defer d.mu.Unlock()

	var lines [][]segment.Segment
	for i := range d.blocks {
		b := &d.blocks[i]
		// Header
		hdr := renderHeader(b)
		lines = append(lines, hdr)

		// Output lines (always shown, not collapsed)
		for _, line := range b.lines {
			lines = append(lines, []segment.Segment{
				segment.NewText("  ", &style.Dim),
				segment.NewText(line, &style.Dim),
			})
		}
		// If the block is running and hasn't produced maxLines yet, pad with
		// blank lines so the block height is stable while running.
		if b.status == blockRunning {
			for j := len(b.lines); j < b.maxLines; j++ {
				lines = append(lines, []segment.Segment{})
			}
		}
	}

	// Crop to terminal height: keep the bottom N lines so finished blocks
	// that scrolled off don't eat terminal real estate.
	maxHeight := opts.Size.Height
	if maxHeight > 0 && len(lines) > maxHeight {
		cropped := lines[len(lines)-maxHeight:]
		var segs []segment.Segment
		for i, line := range cropped {
			if i > 0 {
				segs = append(segs, segment.Segment{Text: "\n"})
			}
			segs = append(segs, line...)
		}
		return segs
	}

	var segs []segment.Segment
	for i, line := range lines {
		if i > 0 {
			segs = append(segs, segment.Segment{Text: "\n"})
		}
		segs = append(segs, line...)
	}
	return segs
}

func renderHeader(b *runBlock) []segment.Segment {
	switch b.status {
	case blockDone:
		s := style.New().WithForeground(style.StandardColor(2)).WithBold(true)
		return []segment.Segment{
			segment.NewText(fmt.Sprintf("✓ %s", b.repo), &s),
			segment.NewText(fmt.Sprintf(" (%s)", b.elapsed.Round(time.Millisecond)), &style.Dim),
		}
	case blockFailed:
		s := style.New().WithForeground(style.StandardColor(1)).WithBold(true)
		return []segment.Segment{
			segment.NewText(fmt.Sprintf("✗ exit %d %s", b.exitCode, b.repo), &s),
			segment.NewText(fmt.Sprintf(" (%s)", b.elapsed.Round(time.Millisecond)), &style.Dim),
		}
	default:
		s := style.New().WithForeground(style.StandardColor(6))
		return []segment.Segment{
			segment.NewText(fmt.Sprintf("→ %s", b.repo), &s),
		}
	}
}

// blockWriter is an io.Writer that buffers partial lines and flushes complete
// lines to the runDisplay. One blockWriter per running repo.
type blockWriter struct {
	idx     int
	display *runDisplay
	buf     []byte
}

func newBlockWriter(idx int, d *runDisplay) *blockWriter {
	return &blockWriter{idx: idx, display: d}
}

func (w *blockWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		line = strings.TrimRight(line, "\r")
		w.display.addLine(w.idx, line)
	}
	return len(p), nil
}

func startLiveDisplay(maxLines int) (*live.Live, *runDisplay) {
	d := newRunDisplay(maxLines)
	l := live.New(
		ui.DefaultConsole,
		d,
		live.WithAutoRefresh(true),
		live.WithRefreshRate(15),
	)
	return l, d
}