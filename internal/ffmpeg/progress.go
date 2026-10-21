package ffmpeg

import (
	"bytes"
	"fmt"
	"io"
	"metarr/internal/domain/logger"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// progressPrefix starts each progress line Metarr prints on stdout for a caller (e.g. Tubarr).
const progressPrefix = "metarr progress: "

// progressWriter reads FFmpeg's -progress output, printing the transcode's progress to out at most once a second.
type progressWriter struct {
	out        io.Writer
	durationUS int64 // 0 if unknown.
	buf        []byte
	vals       map[string]string
	last       time.Time
}

// newProgressWriter returns a progressWriter for an input of durationUS microseconds (0 if unknown).
func newProgressWriter(out io.Writer, durationUS int64) *progressWriter {
	return &progressWriter{out: out, durationUS: durationUS, vals: map[string]string{}}
}

// Write takes FFmpeg's key=value progress lines, printing a progress line at each block's "progress" key.
func (p *progressWriter) Write(b []byte) (int, error) {
	p.buf = append(p.buf, b...)
	for {
		i := bytes.IndexByte(p.buf, '\n')
		if i < 0 {
			break
		}
		key, val, ok := strings.Cut(strings.TrimSpace(string(p.buf[:i])), "=")
		p.buf = p.buf[i+1:]
		if !ok {
			continue
		}
		if key != "progress" {
			p.vals[key] = strings.TrimSpace(val)
			continue
		}
		if val == "end" || time.Since(p.last) >= time.Second {
			p.last = time.Now()
			p.report(val == "end")
		}
	}
	return len(b), nil
}

// report prints the latest progress, as 100% once FFmpeg has finished.
func (p *progressWriter) report(done bool) {
	pct := -1.0
	outUS, err := strconv.ParseInt(p.vals["out_time_us"], 10, 64)
	if err != nil {
		outUS, err = strconv.ParseInt(p.vals["out_time_ms"], 10, 64) // Also microseconds, in older FFmpeg.
	}
	switch {
	case done:
		pct = 100
	case err == nil && p.durationUS > 0:
		pct = min(100, max(0, float64(outUS)*100/float64(p.durationUS)))
	}
	frame, _ := strconv.ParseInt(p.vals["frame"], 10, 64)
	fps, _ := strconv.ParseFloat(p.vals["fps"], 64)
	speed := p.vals["speed"]
	if speed == "N/A" {
		speed = ""
	}
	if _, err := fmt.Fprintf(p.out, "%spct=%.1f frame=%d fps=%.1f speed=%s\n", progressPrefix, pct, frame, fps, speed); err != nil {
		logger.Pl.E("Failed to output FFmpeg progress: %v", err)
	}
}

// probeDurationUS returns a file's duration in microseconds, or 0 if FFprobe can't read it.
func probeDurationUS(inputFile string) int64 {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputFile,
	).Output()
	if err != nil {
		logger.Pl.D(1, "Could not read the duration of %q, progress will have no percentage: %v", inputFile, err)
		return 0
	}
	secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return int64(secs * 1e6)
}
