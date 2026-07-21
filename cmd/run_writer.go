package cmd

import (
	"bytes"
	"strings"
	"sync"

	"github.com/depado/gorich/live"
)

// styledLineWriter is an io.Writer that buffers partial lines, tags them with
// a stream name, and appends markup-styled lines to the runResult and the live
// block display (when active). stdout → dim, stderr → red.
//
// A shared mutex (orderMu) ensures that stdout and stderr lines from the same
// process are appended in arrival order, not arbitrarily interleaved by the
// Go runtime's pipe-reader scheduling.
type styledLineWriter struct {
	mu       sync.Mutex  // protects this writer's partial-line buffer
	orderMu  *sync.Mutex // shared between stdout+stderr writers for the same repo
	buf      []byte
	stream   string // "stdout" or "stderr"
	result   *runResult
	display  *live.BlockDisplay
	blockIdx int
}

func (w *styledLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.buf = append(w.buf, p...)
	var complete []string
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		line = strings.TrimRight(line, "\r")
		complete = append(complete, line)
	}
	w.mu.Unlock()

	if len(complete) > 0 {
		w.orderMu.Lock()
		for _, line := range complete {
			w.result.Lines = append(w.result.Lines, runLine{Stream: w.stream, Text: line})
			if w.display != nil {
				markup := "[dim]" + line + "[/]"
				if w.stream == "stderr" {
					markup = "[red]" + line + "[/]"
				}
				w.display.AppendLine(w.blockIdx, markup)
			}
		}
		w.orderMu.Unlock()
	}
	return len(p), nil
}
