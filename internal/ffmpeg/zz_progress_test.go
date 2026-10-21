package ffmpeg

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestProgressWriter tests FFmpeg's -progress blocks become progress lines, split across writes, at most once a second
// and always at the end.
func TestProgressWriter(t *testing.T) {
	var out bytes.Buffer
	p := newProgressWriter(&out, 10_000_000) // 10 seconds.

	block := "frame=120\nfps=29.97\nout_time_us=2500000\nspeed=1.52x\nprogress=continue\n"
	half := len(block) / 2
	for _, chunk := range []string{block[:half], block[half:]} {
		if _, err := p.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	// A second block straight after is skipped, as under a second has passed.
	if _, err := p.Write([]byte("frame=130\nout_time_us=2700000\nprogress=continue\n")); err != nil {
		t.Fatal(err)
	}
	p.last = time.Now().Add(-2 * time.Second)
	if _, err := p.Write([]byte("frame=200\nfps=30\nout_time_us=N/A\nspeed=N/A\nprogress=continue\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Write([]byte("frame=300\nout_time_us=9990000\nspeed=1.6x\nprogress=end\n")); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"metarr progress: pct=25.0 frame=120 fps=30.0 speed=1.52x eta=4", // 7.5s of video left at 1.52x.
		"metarr progress: pct=-1.0 frame=200 fps=30.0 speed= eta=-1",
		"metarr progress: pct=100.0 frame=300 fps=30.0 speed=1.6x eta=0",
	}
	if got := strings.Split(strings.TrimSpace(out.String()), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestProgressWriterUnknownDuration tests progress has no percentage when the duration is unknown, until FFmpeg ends.
func TestProgressWriterUnknownDuration(t *testing.T) {
	var out bytes.Buffer
	p := newProgressWriter(&out, 0)
	if _, err := p.Write([]byte("frame=10\nout_time_us=500000\nprogress=continue\n")); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "pct=-1.0 frame=10") || !strings.HasSuffix(strings.TrimSpace(got), "eta=-1") {
		t.Errorf("got %q, want no percentage", got)
	}
}
