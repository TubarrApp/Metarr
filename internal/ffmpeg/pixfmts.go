package ffmpeg

import (
	"context"
	"fmt"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/logger"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/TubarrApp/gocommon/sharedconsts"
)

// Pixel formats FFmpeg knows, read once, and those each encoder takes, read once per encoder.
var (
	pixFmtsCache     []string
	pixFmtsCacheOnce sync.Once

	encoderPixFmtsCache = make(map[string][]string)
	encoderPixFmtsMu    sync.Mutex
)

// KnownPixFmt returns FFmpeg's name for a pixel format, adding a missing "le", or an error if FFmpeg has no such format.
func KnownPixFmt(ctx context.Context, name string) (string, error) {
	pixFmtsCacheOnce.Do(func() {
		pixFmtsCache = readPixFmts(ctx)
	})

	// FFmpeg couldn't list them, so it checks the name itself when it runs.
	if len(pixFmtsCache) == 0 {
		return name, nil
	}
	for _, n := range []string{name, name + "le"} {
		if slices.Contains(pixFmtsCache, n) {
			return n, nil
		}
	}
	return "", fmt.Errorf("FFmpeg has no pixel format %q", name)
}

// readPixFmts lists the pixel formats FFmpeg can write.
func readPixFmts(ctx context.Context) []string {
	out, err := exec.CommandContext(ctx, sharedconsts.ProgramFFmpeg, "-hide_banner", "-pix_fmts").Output()
	if err != nil {
		logger.Pl.E("Could not list FFmpeg's pixel formats: %v", err)
		return nil
	}

	// Rows after the "-----" line are flags then name, with 'O' second in the flags for formats FFmpeg can write.
	var names []string
	listed := false
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if !listed {
			listed = len(fields) == 1 && fields[0] == "-----"
			continue
		}
		if len(fields) >= 2 && len(fields[0]) > 1 && fields[0][1] == 'O' {
			names = append(names, fields[1])
		}
	}
	return names
}

// encoderPixFmts lists the pixel formats an encoder takes.
func encoderPixFmts(ctx context.Context, encoder string) []string {
	encoderPixFmtsMu.Lock()
	defer encoderPixFmtsMu.Unlock()
	if pixFmts, ok := encoderPixFmtsCache[encoder]; ok {
		return pixFmts
	}

	// Read the "Supported pixel formats:" line of the encoder's help.
	out, err := exec.CommandContext(ctx, sharedconsts.ProgramFFmpeg, "-hide_banner", "-h", "encoder="+encoder).Output()
	if err != nil {
		logger.Pl.E("Could not read the pixel formats encoder %q takes: %v", encoder, err)
	}
	var pixFmts []string
	for line := range strings.Lines(string(out)) {
		if _, list, found := strings.Cut(line, "Supported pixel formats:"); found {
			pixFmts = strings.Fields(list)
			break
		}
	}
	encoderPixFmtsCache[encoder] = pixFmts
	return pixFmts
}

// encoderPixFmt returns the pixel format to send an encoder for the one asked: that one, else an equivalent it takes.
func encoderPixFmt(ctx context.Context, encoder, asked string) (string, error) {
	supported := encoderPixFmts(ctx, encoder)
	if len(supported) == 0 || slices.Contains(supported, asked) {
		return asked, nil
	}
	for _, equivalent := range consts.PixFmtEquivalents[asked] {
		if slices.Contains(supported, equivalent) {
			logger.Pl.I("Encoder %q doesn't take pixel format %q, sending its equivalent %q.", encoder, asked, equivalent)
			return equivalent, nil
		}
	}
	return "", fmt.Errorf("encoder %q can't encode pixel format %q, it takes: %s", encoder, asked, strings.Join(supported, " "))
}

// pixFmtMatches reports whether a video in a codec is already in the pixel format asked, going by the one it reads as.
func pixFmtMatches(codec, asked, source string) bool {
	// Decoders report these as another format: H.264's gray as 4:2:0, and VP8 and VP9's alpha is stored apart from the picture.
	if (codec == sharedconsts.VCodecH264 && strings.HasPrefix(asked, "gray")) || asked == "yuva420p" {
		return true
	}

	// Full range formats have the same bit depth and chroma.
	asked = strings.Replace(asked, "yuvj", "yuv", 1)
	source = strings.Replace(source, "yuvj", "yuv", 1)
	return asked == source || slices.Contains(consts.PixFmtEquivalents[asked], source)
}
