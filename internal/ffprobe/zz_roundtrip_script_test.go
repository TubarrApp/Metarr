package ffprobe

import (
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"metarr/internal/parsing"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedtags"
)

// TestWriteRoundTripScript writes a shell script repeating TestRoundTripWithFFmpeg, for machines without Go.
func TestWriteRoundTripScript(t *testing.T) {
	path := os.Getenv("METARR_ROUNDTRIP_SCRIPT")
	if path == "" {
		t.Skip("set METARR_ROUNDTRIP_SCRIPT to the script path to write")
	}

	flatKey := regexp.MustCompile(`[^A-Za-z0-9]`)
	var b strings.Builder
	b.WriteString(`#!/bin/sh
# Writes every container with each tag Metarr maps, then checks ffprobe reads each one back.
set -u
dir=$(mktemp -d) && cd "$dir" || exit 1
ffmpeg -version | head -n 1
ffmpeg -v error -f lavfi -i color=s=176x144:d=0.2 src.mkv || exit 1
fails=0

# expect <ext> <ffprobe flat output> <section.key> <value prefix>
expect() {
	if printf '%s\n' "$2" | grep -iqF "$3=\"$4"; then :; else
		echo "FAIL $1: $3 is not \"$4\" (got: $(printf '%s\n' "$2" | grep -iF "$3=" | head -n 1))"
		fails=$((fails+1))
	fi
}
`)

	exts := slices.Sorted(maps.Keys(sharedconsts.AllVidExtensions))
	keys := slices.Sorted(maps.Keys(roundTripTags))
	for _, ext := range exts {
		fmt.Fprintf(&b, "\n# %s\n", ext)

		// Write the container.
		b.WriteString("ffmpeg -v error -y -i src.mkv")
		for _, key := range keys {
			if tag := parsing.GetContainerKeys(key, ext); tag != "" {
				fmt.Fprintf(&b, " -metadata '%s=%s'", tag, roundTripTags[key])
			}
		}
		switch ext {
		case sharedconsts.ExtOGM:
			b.WriteString(" -f ogv")
		case sharedconsts.ExtRMVB:
			b.WriteString(" -f rm")
		}
		fmt.Fprintf(&b, " out%s || { echo \"FAIL %s: ffmpeg could not write it\"; fails=$((fails+1)); }\n", ext, ext)

		// 3GP tags can't be read back.
		if ext == sharedconsts.Ext3GP || ext == sharedconsts.Ext3G2 {
			continue
		}

		// Read it back from the section FFprobe reports the container's tags in.
		section := "format.tags."
		switch ext {
		case sharedconsts.ExtMTS, sharedconsts.ExtTS:
			section = "programs.program.0.tags."
		case sharedconsts.ExtOGM, sharedconsts.ExtOGV:
			section = "streams.stream.0.tags."
		}
		fmt.Fprintf(&b, "p=$(ffprobe -v quiet -show_entries format_tags:stream_tags:program_tags -of flat out%s)\n", ext)
		for _, key := range keys {
			tag := parsing.GetContainerKeys(key, ext)
			if tag == "" {
				continue
			}
			value := roundTripTags[key]
			if slices.Contains(sharedtags.DateKeys, key) {
				value = getDatePart(value)
			}
			fmt.Fprintf(&b, "expect %s \"$p\" '%s%s' '%s'\n", ext, section, flatKey.ReplaceAllString(probeKey(tag, ext), "_"), value)
		}
	}

	// Check the .ogm and .rmvb muxers take Metarr's codecs.
	b.WriteString(`
# Codecs through the .ogm and .rmvb muxers.
ffmpeg -v error -y -f lavfi -i color=s=176x144:d=0.2 -f lavfi -i sine=d=0.2 -c:v libvpx -c:a libvorbis -f ogv codecs.ogm || { echo "FAIL .ogm: vp8 + vorbis"; fails=$((fails+1)); }
ffmpeg -v error -y -f lavfi -i color=s=176x144:d=0.2 -f lavfi -i sine=d=0.2 -c:v libx264 -c:a aac -f rm codecs.rmvb || { echo "FAIL .rmvb: h264 + aac"; fails=$((fails+1)); }

echo "Done: $fails failures"
rm -rf "$dir"
`)

	if err := os.WriteFile(path, []byte(b.String()), 0o755); err != nil {
		t.Fatal(err)
	}
}
