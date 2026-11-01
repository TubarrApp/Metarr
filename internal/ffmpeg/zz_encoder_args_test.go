package ffmpeg

import (
	"slices"
	"strings"
	"testing"

	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// TestSetVideoEncoderArgs tests that each target codec's args are converted for the encoder that actually runs.
func TestSetVideoEncoderArgs(t *testing.T) {
	t.Cleanup(func() { abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{}) })
	abstractions.Set(keys.TranscodeVideoArgsMap, map[string]sharedmodels.EncoderArgs{
		sharedconsts.VCodecHEVC: {Quality: "24", PixFmt: "yuv420p10le", VideoFilter: "scale=1280:-2", Args: []string{"-preset", "slow"}},
		sharedconsts.VCodecAV1:  {Quality: "60"},
		sharedconsts.VCodecVP9:  {PixFmt: "yuv444p10le"},
		sharedconsts.VCodecH264: {PixFmt: "yuv422p"},
	})

	// The pixel formats each encoder takes, as FFmpeg 9 lists them.
	encoderPixFmtsMu.Lock()
	encoderPixFmtsCache["libx265"] = []string{"yuv420p", "yuv422p", "yuv444p", "yuv420p10le", "yuv422p10le", "yuv444p10le"}
	encoderPixFmtsCache["libsvtav1"] = []string{"yuv420p", "yuv420p10le"}
	encoderPixFmtsCache["hevc_nvenc"] = []string{"yuv420p", "nv12", "p010le", "yuv444p", "nv16", "p210le", "yuv444p16le"}
	encoderPixFmtsCache["vp9_qsv"] = []string{"nv12", "p010le", "vuyx", "xv30le"}
	encoderPixFmtsCache["h264_qsv"] = []string{"nv12"}
	encoderPixFmtsMu.Unlock()
	software := func(encoder string) []string { return []string{consts.FFmpegCV0, encoder} }

	tests := []struct {
		name       string
		accelType  string
		gpuCodec   []string
		softCodec  []string
		desired    string
		wantArgs   []string
		wantFilter string
		wantErr    bool
	}{
		{"software", "", nil, software("libx265"), sharedconsts.VCodecHEVC,
			[]string{consts.FFmpegCRF, "24", consts.FFmpegPixFmt, "yuv420p10le", "-preset", "slow"}, "scale=1280:-2", false},
		{"copied video gets nothing", "", nil, software(consts.FFVCodecKeyCopy), sharedconsts.VCodecHEVC, nil, "", false},
		{"vaapi joins the upload filter", sharedconsts.AccelTypeVAAPI, software("hevc_vaapi"), nil, sharedconsts.VCodecHEVC,
			[]string{"-qp", "24", "-preset", "slow"}, "scale=1280:-2,format=p010,hwupload", false},
		{"vaapi without args still uploads", sharedconsts.AccelTypeVAAPI, software("mpeg2_vaapi"), nil, sharedconsts.VCodecMPEG2,
			nil, "format=nv12,hwupload", false},
		{"vaapi refuses formats it can't upload", sharedconsts.AccelTypeVAAPI, software("vp9_vaapi"), nil, sharedconsts.VCodecVP9,
			nil, "", true},
		{"vaapi fallen back to software has no upload filter", sharedconsts.AccelTypeVAAPI, nil, software("libx265"), sharedconsts.VCodecHEVC,
			[]string{consts.FFmpegCRF, "24", consts.FFmpegPixFmt, "yuv420p10le", "-preset", "slow"}, "scale=1280:-2", false},
		{"nvenc gets the equivalent 10-bit format", sharedconsts.AccelTypeCuda, software("hevc_nvenc"), nil, sharedconsts.VCodecHEVC,
			[]string{"-rc", "vbr", "-cq", "24", consts.FFmpegPixFmt, "p010le", "-preset", "slow"}, "scale=1280:-2", false},
		{"qsv gets the equivalent 4:4:4 format", sharedconsts.AccelTypeQSV, software("vp9_qsv"), nil, sharedconsts.VCodecVP9,
			[]string{consts.FFmpegPixFmt, "xv30le"}, "", false},
		{"encoder with no such format", sharedconsts.AccelTypeQSV, software("h264_qsv"), nil, sharedconsts.VCodecH264, nil, "", true},
		{"software encoder with no such format", "", nil, software("libsvtav1"), sharedconsts.VCodecVP9, nil, "", true},
		{"gpu quality capped at 51", sharedconsts.AccelTypeCuda, software("av1_nvenc"), nil, sharedconsts.VCodecAV1,
			[]string{"-rc", "vbr", "-cq", "51"}, "", false},
		{"software keeps AV1's higher quality", "", nil, software("libsvtav1"), sharedconsts.VCodecAV1,
			[]string{consts.FFmpegCRF, "60"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &ffCommandBuilder{builder: &strings.Builder{}, videoCodecGPU: tt.gpuCodec, videoCodecSoftware: tt.softCodec}
			err := b.setVideoEncoderArgs(t.Context(), tt.accelType, tt.desired)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got := slices.Concat(b.qualityParameter, b.encoderArgs); !slices.Equal(got, tt.wantArgs) || b.videoFilter != tt.wantFilter {
				t.Errorf("got args %q, filter %q; want %q, %q", got, b.videoFilter, tt.wantArgs, tt.wantFilter)
			}
		})
	}
}

// TestSetGPUAccelerationCodecCopies tests that a GPU encoder is only chosen when the video changes codec.
func TestSetGPUAccelerationCodecCopies(t *testing.T) {
	const encoders = " hevc_vaapi h264_vaapi "
	for _, desired := range []string{sharedconsts.VCodecHEVC, sharedconsts.VCodecCopy, ""} {
		b := &ffCommandBuilder{builder: &strings.Builder{}}
		b.setGPUAccelerationCodec(sharedconsts.AccelTypeVAAPI, sharedconsts.VCodecHEVC, desired, encoders)
		if b.videoCodecGPU != nil {
			t.Errorf("HEVC video wanting %q: got GPU codec %q, want a copy", desired, b.videoCodecGPU)
		}
	}

	b := &ffCommandBuilder{builder: &strings.Builder{}}
	b.setGPUAccelerationCodec(sharedconsts.AccelTypeVAAPI, sharedconsts.VCodecH264, sharedconsts.VCodecHEVC, encoders)
	if want := []string{consts.FFmpegCV0, "hevc_vaapi"}; !slices.Equal(b.videoCodecGPU, want) {
		t.Errorf("H.264 video wanting HEVC: got GPU codec %q, want %q", b.videoCodecGPU, want)
	}
}

// TestBuildFinalCommandGPUNode tests that the GPU device and video filter are only added when the GPU encodes.
func TestBuildFinalCommandGPUNode(t *testing.T) {
	b := &ffCommandBuilder{
		builder:    &strings.Builder{},
		inputFile:  "in.mkv",
		outputFile: "out.mkv",
		gpuNode:    []string{consts.FFmpegDeviceVAAPI, "/dev/dri/renderD128"},
	}
	args, err := b.buildFinalCommand(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(args, consts.FFmpegDeviceVAAPI) || slices.Contains(args, consts.FFmpegFilter) {
		t.Errorf("copy run: got %q, want no GPU device or filter", args)
	}

	const upload = "format=nv12,hwupload"
	b.videoCodecGPU = []string{consts.FFmpegCV0, "hevc_vaapi"}
	b.videoFilter = upload
	if args, err = b.buildFinalCommand(nil, false); err != nil {
		t.Fatal(err)
	}
	filter := slices.Index(args, consts.FFmpegFilter)
	if !slices.Contains(args, consts.FFmpegDeviceVAAPI) || filter < 0 || filter+1 >= len(args) || args[filter+1] != upload {
		t.Errorf("GPU run: got %q, want the GPU device and %s %s", args, consts.FFmpegFilter, upload)
	}
}
