// Package ffmpeg handles FFmpeg command building and execution.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/vars"
	"metarr/internal/file"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
)

// ExecuteVideo writes metadata to a single video file.
func ExecuteVideo(ctx context.Context, fd *models.FileData) error {
	var tmpOutPath string

	origPath := fd.OriginalVideoPath
	origExt := filepath.Ext(origPath)

	// Output extension: the one set, else the video's own, or a writable one if FFmpeg can't write its own.
	outExt, replaced := parsing.OutputExt(origPath)

	// Get current codecs.
	currentVCodec, currentACodec, err := checkCodecs(fd.OriginalVideoPath)
	if err != nil {
		logger.Pl.E("Failed to check input file %q codec: %v", fd.OriginalVideoPath, err)
	}

	// Check codec mismatches.
	desiredVCodec := getOutputVideoCodecString(currentVCodec)
	desiredACodec := getOutputAudioCodecString(currentACodec)

	// A video FFmpeg can't rewrite is left alone unless it needs changing, then written anew beside it.
	if replaced {
		if skipProcessing(fd, currentVCodec, currentACodec, desiredVCodec, desiredACodec, origExt) {
			return nil
		}
		logger.Pl.W("FFmpeg can't write %s files that play, so %q will be written as a %s file, keeping the original", origExt, origPath, outExt)
	}

	// Make sure the output container can hold the codecs.
	if desiredVCodec, desiredACodec, err = fitCodecsToContainer(ctx, origExt, outExt, currentVCodec.Codec, desiredVCodec, currentACodec.Codec, desiredACodec); err != nil {
		return err
	}

	// Return early if no processing is needed.
	if skipProcessing(fd, currentVCodec, currentACodec, desiredVCodec, desiredACodec, outExt) {
		return nil
	}
	logger.Pl.I("Will execute video from extension %q → %q", origExt, outExt)

	// Set temp output path: DO NOT use user cache or temp dir, can cause cross-device link move failures.
	fileBase := parsing.GetBaseNameWithoutExt(origPath)
	tmpOutPath = filepath.Join(fd.VideoDirectory, sharedconsts.TempTag+fileBase+origExt+outExt)
	logger.Pl.D(3, "Orig ext: %q, Out ext: %q", origExt, outExt)

	// Remove temp file on function end.
	defer func() {
		if _, err := os.Stat(tmpOutPath); err == nil {
			if err := os.Remove(tmpOutPath); err != nil {
				logger.Pl.E("Failed to remove %q: %v", tmpOutPath, err)
			}
		}
	}()

	// Set post-FFmpeg video path in model.
	baseName := parsing.GetBaseNameWithoutExt(origPath)
	fd.PostFFmpegVideoPath = filepath.Join(fd.VideoDirectory, baseName) + outExt

	logger.Pl.I("Video file path data:\n\nOriginal Video Path: %s\nMetadata File Path: %s\nPost-FFmpeg Video Path: %s\n\nTemp Output Path: %s", origPath,
		fd.MetaFilePath,
		fd.PostFFmpegVideoPath,
		tmpOutPath)

	// Run the FFmpeg command (n total attempts).
	maxAttempts := 3
	durationUS := probeDurationUS(origPath)
	builder, err := newFfCommandBuilder(fd, tmpOutPath)
	if err != nil {
		return err
	}

	for i := 1; i <= maxAttempts; i++ {
		// Build command.
		args, err := builder.buildCommand(ctx, fd, desiredVCodec, desiredACodec, outExt)
		if err != nil {
			return err
		}
		args = append([]string{"-progress", "pipe:1", "-nostats"}, args...)
		command := exec.CommandContext(ctx, "ffmpeg", args...)
		logger.Pl.I("Constructed FFmpeg command for %q:\n\n%v\n", fd.OriginalVideoPath, command.String())

		// Progress goes to stdout as progress lines, the rest of FFmpeg's output to stderr.
		var stderr bytes.Buffer
		command.Stdout = newProgressWriter(os.Stdout, durationUS)
		command.Stderr = io.MultiWriter(os.Stderr, &stderr)

		// Run command.
		logger.Pl.P("%s!!! Starting FFmpeg attempt %d for %q...\n%s", sharedconsts.ColorCyan, i, baseName, sharedconsts.ColorReset)
		if err := command.Run(); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("ffmpeg canceled for %q: %w", baseName, ctx.Err())
			}
			// Exit if final attempt errored (the caller records the error).
			if i == maxAttempts {
				return fmt.Errorf("ffmpeg failed after %d attempts for %q due to error: %w\n\nCaptured output:\n%s", maxAttempts, baseName, err, stderr.String())
			}

			// Retry.
			logger.Pl.E("Command attempt %d failed for %q due to error: %v:\n\nCaptured output:\n%s", i, baseName, err, stderr.String())
			continue
		}
		// FFmpeg completed successfully: Exit loop.
		logger.Pl.S("FFmpeg successfully processed %q", baseName)
		break
	}

	// Back up the original if the new video takes its path, unless it's purged.
	samePath := origPath == fd.PostFFmpegVideoPath
	purgeOriginal := abstractions.GetBool(keys.PurgeOriginalVideo)
	if samePath && (!purgeOriginal || abstractions.GetBool(keys.NoFileOverwrite)) {
		if fd.BackupVideoPath, err = makeBackup(origPath); err != nil {
			return err
		}
	}
	if !samePath {
		if _, err := os.Stat(fd.PostFFmpegVideoPath); err == nil {
			return fmt.Errorf("output %q already exists, not overwriting it", fd.PostFFmpegVideoPath)
		}
	}

	// Move the temp file into place, then remove the original if its extension differed and it's purged.
	if err := os.Rename(tmpOutPath, fd.PostFFmpegVideoPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	models.ReportVideoPath(fd.PostFFmpegVideoPath)
	if !samePath && !replaced && purgeOriginal {
		logger.Pl.I("Original file not type %s, removing %q", outExt, origPath)
		if err := os.Remove(origPath); err != nil {
			vars.AddToErrorArray(err)
			return fmt.Errorf("failed to remove original file (%s). Error: %w", origPath, err)
		}
	}

	// Log success.
	fmt.Fprintf(os.Stderr, "\n")
	logger.Pl.S("Successfully processed video:\n\nOriginal file: %s\nNew file: %s\n\nTitle: %s", origPath,
		fd.PostFFmpegVideoPath,
		fd.MTitleDesc.Title)

	return nil
}

// skipProcessing determines whether the program should process this video (meta already exists, file extensions are unchanged, and codecs match).
func skipProcessing(fd *models.FileData, currentVCodec, currentACodec sharedmodels.FromCodec, desiredVCodec, desiredACodec, outExt string) (skipProcessing bool) {
	logger.Pl.I("Checking if processing should continue for file %q...", fd.OriginalVideoPath)

	// Write thumbnail.
	if models.ForceThumbnailWrite() && fd.EmbedThumbnail() != "" {
		logger.Pl.I("Thumbnail detected. Will write to file.")
		return false
	}

	var (
		differentExt, codecsDiffer bool
	)

	// Check for extension difference.
	currentExt := strings.ToLower(filepath.Ext(fd.OriginalVideoPath))
	if currentExt != outExt && outExt != "" {
		logger.Pl.I("Current extension (%s) differs from output extension (%s) in file %q, will convert...", currentExt, outExt, fd.OriginalVideoPath)
		differentExt = true
	}

	// Check for codec difference.
	if desiredVCodec != currentVCodec.Codec && desiredVCodec != "" && desiredVCodec != sharedconsts.VCodecCopy {
		logger.Pl.I("Current video codec (%s) differs from desired video codec (%s) in file %q, will convert...", currentVCodec.Codec, desiredVCodec, fd.OriginalVideoPath)
		codecsDiffer = true
	}
	if desiredACodec != currentACodec.Codec && desiredACodec != "" && desiredACodec != sharedconsts.ACodecCopy {
		logger.Pl.I("Current audio codec (%s) differs from desired audio codec (%s) in file %q, will convert...", currentACodec, desiredACodec, fd.OriginalVideoPath)
		codecsDiffer = true
	}

	// Check if metadata already exists.
	if !fd.MetaAlreadyExists {
		logger.Pl.I("Desired metadata or thumbnail differs from data extracted from original file (%s), will process...", fd.OriginalVideoPath)
	}

	// Final checks.
	if !codecsDiffer && !differentExt && fd.MetaAlreadyExists {
		// -- SKIP FURTHER PROCESSING. --
		logger.Pl.I("For file %q, all metadata exists, codecs match, and extensions match. Skipping processing...", fd.OriginalVideoPath)

		// Save 'post-FFmpeg' video path into model.
		fd.PostFFmpegVideoPath = filepath.Join(fd.VideoDirectory, parsing.GetBaseNameWithoutExt(fd.OriginalVideoPath)) + outExt

		// Set final paths and end processing.
		fd.SetFinalPaths(fd.OriginalVideoPath, fd.MetaFilePath)
		return true
	}

	logger.Pl.I("Metadata, codec, or file extension mismatch. Continuing to process file %q", fd.OriginalVideoPath)
	return false
}

// makeBackup renames the original to a backup, returning its path ("" if none was made).
func makeBackup(origPath string) (backupPath string, err error) {
	origInfo, err := os.Stat(origPath)
	if os.IsNotExist(err) {
		logger.Pl.I("File does not exist, safe to proceed overwriting: %s", origPath)
		return "", nil
	}

	// Rename original to backup.
	backupPath, err = file.RenameToBackup(origPath)
	if err != nil {
		return "", fmt.Errorf("failed to rename original file and preserve file is on, aborting: %w", err)
	}
	if backupPath == "" {
		logger.Pl.I("Identical backup of %q already exists, skipping backup", origPath)
		return "", nil
	}

	// Verify backup exists and matches size.
	backInfo, err := os.Stat(backupPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("backup file %q was not created, aborting", backupPath)
	}
	if (origInfo != nil && backInfo != nil) && (origInfo.Size() != backInfo.Size()) {
		return "", fmt.Errorf("backup file size %d does not match original %d, aborting", origInfo.Size(), backInfo.Size())
	}

	return backupPath, nil
}

// checkCodecs checks the input codec to determine if a straight remux is possible.
func checkCodecs(inputFile string) (videoCodec, audioCodec sharedmodels.FromCodec, err error) {
	blankFromCodec := sharedmodels.NewFromCodec(sharedconsts.CodecCopy, "")
	if inputFile == "" {
		return blankFromCodec, blankFromCodec, fmt.Errorf("input file is empty, cannot check codecs")
	}

	// Determine codecs and video compression level.
	if audioCodec.Codec, videoCodec.Codec, audioCodec.Compression, videoCodec.Compression, err = getInputCodecs(inputFile); err != nil {
		return blankFromCodec, blankFromCodec, err
	}

	// Log and return detected codecs.
	logger.Pl.D(1, "Detected codecs - video: %s, audio: %s", videoCodec.Codec, audioCodec.Codec)
	return videoCodec, audioCodec, nil
}

// ffprobeBPPPFOutput holds info used to grab codecs and calculate BPPPF (compression level).
type ffprobeCodecOutput struct {
	Streams []ffprobeCodecOutputStream `json:"streams"`
}

// ffprobeCodecOutputStream holds the info grabbed for the stream.
type ffprobeCodecOutputStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
	BitRate      string `json:"bit_rate"`
	SampleRate   string `json:"sample_rate"`
	Channels     int    `json:"channels"`
}

// getInputCodecs determines the codecs and compression level of the video and audio through FFprobe.
func getInputCodecs(inputFile string) (audioCodec, videoCodec string, audioCompression, videoCompression sharedconsts.CompressionLevel, err error) {
	if inputFile == "" {
		return "", "", sharedconsts.CompressionBlank, sharedconsts.CompressionBlank, fmt.Errorf("input file sent in empty")
	}

	// FFprobe command to grab relevant values.
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries",
		"stream=codec_type,codec_name,width,height,avg_frame_rate,bit_rate,sample_rate,channels",
		"-of", "json",
		inputFile,
	)

	// Run.
	raw, err := cmd.Output()
	if err != nil {
		return "", "", sharedconsts.CompressionBlank, sharedconsts.CompressionBlank, err
	}

	var vInfo ffprobeCodecOutput
	if err := json.Unmarshal(raw, &vInfo); err != nil {
		return "", "", sharedconsts.CompressionBlank, sharedconsts.CompressionBlank, err
	}

	// Find the first video and audio streams.
	var video *ffprobeCodecOutputStream
	var audio *ffprobeCodecOutputStream
	for i := range vInfo.Streams {
		stream := &vInfo.Streams[i]

		switch stream.CodecType {
		case "video":
			if video == nil {
				video = stream
				videoCodec = strings.TrimSpace(video.CodecName)
				logger.Pl.I("Got video codec from '%s' as '%s'", inputFile, videoCodec)
			}
		case "audio":
			if audio == nil {
				audio = stream
				audioCodec = strings.TrimSpace(audio.CodecName)
				logger.Pl.I("Got audio codec from '%s' as '%s'", inputFile, audioCodec)
			}
		}
	}
	if video == nil {
		return "", "", sharedconsts.CompressionBlank, sharedconsts.CompressionBlank, fmt.Errorf("no video stream found")
	}

	// Determine video compression level.
	videoCompression, err = getVideoCompressionLevel(video, inputFile)
	if err != nil {
		return audioCodec, videoCodec, sharedconsts.CompressionBlank, sharedconsts.CompressionBlank, err
	}

	// Determine audio compression level.
	if audio != nil {
		audioCompression, err = getAudioCompressionLevel(audio, inputFile)
		if err != nil {
			return audioCodec, videoCodec, audioCompression, videoCompression, err
		}
	}

	return audioCodec, videoCodec, audioCompression, videoCompression, nil
}

// getVideoCompressionLevel determines the compression level of a video stream through BPPPF.
func getVideoCompressionLevel(video *ffprobeCodecOutputStream, inputFile string) (sharedconsts.CompressionLevel, error) {
	// Validate video dimensions.
	if video.Width <= 0 || video.Height <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid video dimensions: %dx%d", video.Width, video.Height)
	}

	// Parse average frame rate.
	numerator, denominator, ok := strings.Cut(video.AvgFrameRate, "/")
	if !ok {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid average frame rate %q", video.AvgFrameRate)
	}

	num, err := strconv.ParseFloat(numerator, 64)
	if err != nil {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid average frame rate %q: %w", video.AvgFrameRate, err)
	}

	den, err := strconv.ParseFloat(denominator, 64)
	if err != nil || den <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid average frame rate %q", video.AvgFrameRate)
	}

	frameRate := num / den
	if frameRate <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid average frame rate %q", video.AvgFrameRate)
	}

	// Parse video bitrate.
	bitRate, err := strconv.ParseFloat(video.BitRate, 64)
	if err != nil || bitRate <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid video bitrate %q", video.BitRate)
	}

	// Calculate BPPPF.
	bpppf := bitRate / (float64(video.Width) * float64(video.Height) * frameRate)

	// Determine compression level.
	compressionLevel := "low compression"
	var compression sharedconsts.CompressionLevel
	switch {
	case bpppf < sharedconsts.BPPPFHighCompressionThreshold:
		compression = sharedconsts.CompressionHigh
		compressionLevel = "high compression"
	case bpppf < sharedconsts.BPPPFMediumCompressionThreshold:
		compression = sharedconsts.CompressionMedium
		compressionLevel = "average compression"
	default:
		compression = sharedconsts.CompressionLow
	}

	logger.Pl.I("Video '%s''s stream bit rate (%f), dimensions (%dx%d), and frame rate (%f) determine the video has %s", inputFile, bitRate, video.Width, video.Height, frameRate, compressionLevel)
	return compression, nil
}

// getAudioCompressionLevel determines the compression level of an audio stream.
func getAudioCompressionLevel(audio *ffprobeCodecOutputStream, inputFile string) (sharedconsts.CompressionLevel, error) {
	if audio == nil {
		return sharedconsts.CompressionBlank, fmt.Errorf("audio stream is nil")
	}

	// Parse audio sample rate.
	sampleRate, err := strconv.ParseFloat(audio.SampleRate, 64)
	if err != nil || sampleRate <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid audio sample rate %q", audio.SampleRate)
	}

	// Validate audio channels.
	if audio.Channels <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid audio channel count %d", audio.Channels)
	}

	// Parse audio bitrate.
	bitRate, err := strconv.ParseFloat(audio.BitRate, 64)
	if err != nil || bitRate <= 0 {
		return sharedconsts.CompressionBlank, fmt.Errorf("invalid audio bitrate %q", audio.BitRate)
	}

	// Calculate bits per second per channel, normalised to 48 kHz.
	bpspc := bitRate / float64(audio.Channels) * (48000 / sampleRate)

	// Determine compression level.
	compressionLevel := "low compression"
	var compression sharedconsts.CompressionLevel
	switch {
	case bpspc < sharedconsts.AudioBPSPCHighCompressionThreshold:
		compression = sharedconsts.CompressionHigh
		compressionLevel = "high compression"
	case bpspc < sharedconsts.AudioBPSPCMediumCompressionThreshold:
		compression = sharedconsts.CompressionMedium
		compressionLevel = "average compression"
	default:
		compression = sharedconsts.CompressionLow
	}

	logger.Pl.I("Video '%s's audio stream bit rate (%f), sample rate (%s), and channels (%d) determine the file has %s", inputFile, bitRate, audio.SampleRate, audio.Channels, compressionLevel)
	return compression, nil
}
