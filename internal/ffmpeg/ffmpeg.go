// Package ffmpeg handles FFmpeg command building and execution.
package ffmpeg

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/vars"
	"metarr/internal/ffprobe"
	"metarr/internal/file"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"metarr/internal/report"
	"os"
	"path/filepath"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedmodels"
	"github.com/TubarrApp/gocommon/sharedvalidation"
)

// ExecuteVideo writes metadata to a single video file.
func ExecuteVideo(ctx context.Context, fd *models.FileData) error {
	// Output extension: the one set, else the video's own, or a writable one if FFmpeg can't write its own.
	outputFileExt, replaced := parsing.OutputExt(fd.OriginalVideoPath)

	// Get current codecs, once for working out the targets and building the command.
	inputFileVCodec, inputFileACodec, err := getCodecs(ctx, fd.OriginalVideoPath)
	if err != nil {
		logger.Pl.E("Failed to check input file %q codec: %v", fd.OriginalVideoPath, err)
	}

	// Work out the codecs, stopping here if the video needs no processing.
	targetVCodec, targetACodec, reencodeV, reencodeA, skip, err := prepareCodecs(ctx, fd, inputFileVCodec, inputFileACodec, outputFileExt, replaced)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}

	// Set the paths.
	tmpOutPath := prepareOutputPaths(fd, outputFileExt)

	// Remove temp file on function end.
	defer func() {
		if _, err := os.Stat(tmpOutPath); err == nil {
			if err := os.Remove(tmpOutPath); err != nil {
				logger.Pl.E("Failed to remove %q: %v", tmpOutPath, err)
			}
		}
	}()

	// Write the new video to the temp path, then move it into place.
	if err := executeFFmpeg(ctx, fd, tmpOutPath, inputFileVCodec, inputFileACodec, targetVCodec, targetACodec, outputFileExt, reencodeV, reencodeA); err != nil {
		return err
	}
	return finalizeOutput(fd, tmpOutPath, outputFileExt, replaced)
}

// prepareCodecs returns the codecs to write a video in and whether its video and audio are encoded again in their own, or whether it needs no processing at all.
func prepareCodecs(ctx context.Context, fd *models.FileData, inputFileVCodec, inputFileACodec sharedmodels.FromCodec, outputFileExt string, replaced bool) (targetVCodec, targetACodec string, reencodeV, reencodeA, skip bool, err error) {
	inputFilePath := fd.OriginalVideoPath
	inputFileExt := filepath.Ext(inputFilePath)

	// Check codec mismatches.
	targetVCodec = getOutputCodecString(keys.TranscodeVideoCodecMap, inputFileVCodec, sharedconsts.Video)
	targetACodec = getOutputCodecString(keys.TranscodeAudioCodecMap, inputFileACodec, sharedconsts.Audio)

	// Video and audio already in the target output codec may still need encoding.
	reencodeV = reencodeVideo(ctx, inputFilePath, inputFileVCodec)
	reencodeA = reencodeAudio(ctx, inputFilePath, inputFileACodec)

	// A video FFmpeg can't rewrite is left alone unless it needs changing, then written anew beside it.
	if replaced {
		if !reencodeV && !reencodeA && skipProcessing(fd, inputFileVCodec, inputFileACodec, targetVCodec, targetACodec, inputFileExt) {
			return "", "", false, false, true, nil
		}
		logger.Pl.W("FFmpeg can't write %s files that play, so %q will be written as a %s file, keeping the original", inputFileExt, inputFilePath, outputFileExt)
	}

	// Make sure the output container can hold the codecs.
	if targetVCodec, err = fitVideoCodecToContainer(ctx, inputFilePath, outputFileExt, inputFileVCodec.Codec, targetVCodec); err != nil {
		return "", "", false, false, false, err
	}
	if targetACodec, err = fitAudioCodecToContainer(ctx, inputFilePath, outputFileExt, inputFileACodec.Codec, targetACodec); err != nil {
		return "", "", false, false, false, err
	}

	// A video encoded again with no codec named stays in its own, which the container was just checked for.
	if reencodeV && (targetVCodec == "" || targetVCodec == sharedconsts.VCodecCopy) {
		targetVCodec = inputFileVCodec.Codec
	}

	// Audio encoded again with no codec named stays in its own too.
	if reencodeA && (targetACodec == "" || targetACodec == sharedconsts.ACodecCopy) {
		targetACodec = inputFileACodec.Codec
	}

	// Return early if no processing is needed.
	if !reencodeV && !reencodeA && skipProcessing(fd, inputFileVCodec, inputFileACodec, targetVCodec, targetACodec, outputFileExt) {
		return "", "", false, false, true, nil
	}
	return targetVCodec, targetACodec, reencodeV, reencodeA, false, nil
}

// prepareOutputPaths sets the video's post-FFmpeg path in its model, and returns the temp path FFmpeg writes to.
func prepareOutputPaths(fd *models.FileData, outputFileExt string) (tmpOutPath string) {
	inputFilePath := fd.OriginalVideoPath
	inputFileExt := filepath.Ext(inputFilePath)
	logger.Pl.I("Will execute video from extension %q → %q", inputFileExt, outputFileExt)

	// Set temp output path: DO NOT use user cache or temp dir, can cause cross-device link move failures.
	baseName := parsing.GetBaseNameWithoutExt(inputFilePath)
	tmpOutPath = filepath.Join(fd.VideoDirectory, tempOutputName(baseName, inputFileExt, outputFileExt))
	logger.Pl.D(3, "Input file ext: %q, Output ext: %q", inputFileExt, outputFileExt)

	// Set post-FFmpeg video path in model.
	fd.PostFFmpegVideoPath = filepath.Join(fd.VideoDirectory, baseName) + outputFileExt

	logger.Pl.I("Video file path data:\n\nOriginal Video Path: %s\nMetadata File Path: %s\nPost-FFmpeg Video Path: %s\n\nTemp Output Path: %s", inputFilePath,
		fd.MetaFilePath,
		fd.PostFFmpegVideoPath,
		tmpOutPath)

	return tmpOutPath
}

// executeFFmpeg runs FFmpeg to write the new video at tmpOutPath, trying again if it fails.
func executeFFmpeg(ctx context.Context, fd *models.FileData, tmpOutPath string, inputFileVCodec, inputFileACodec sharedmodels.FromCodec, targetVCodec, targetACodec, outputFileExt string, reencodeV, reencodeA bool) error {
	inputFilePath := fd.OriginalVideoPath
	baseName := parsing.GetBaseNameWithoutExt(inputFilePath)

	// Run the FFmpeg command (n total attempts).
	maxAttempts := 3
	durationUS := ffprobe.ProbeDurationUS(inputFilePath)
	builder, err := newFfCommandBuilder(fd, tmpOutPath)
	if err != nil {
		return err
	}
	defer builder.removeTempThumbnails()

	for i := 1; i <= maxAttempts; i++ {
		// Build command.
		args, err := builder.buildCommand(ctx, fd, inputFileVCodec, inputFileACodec, targetVCodec, targetACodec, outputFileExt, reencodeV, reencodeA)
		if err != nil {
			return err
		}

		// Run command, once per encoding pass.
		logger.Pl.P("%s!!! Starting FFmpeg attempt %d for %q...\n%s", sharedconsts.ColorCyan, i, baseName, sharedconsts.ColorReset)
		if stderr, err := runPasses(ctx, args, builder.passes, durationUS, inputFilePath); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("ffmpeg canceled for %q: %w", baseName, ctx.Err())
			}
			// Exit if final attempt errored (the caller records the error).
			if i == maxAttempts {
				return fmt.Errorf("ffmpeg failed after %d attempts for %q due to error: %w: %s\n\nCaptured output:\n%s", maxAttempts, baseName, err,
					cmp.Or(ffmpegFailure(stderr), "FFmpeg gave no reason"), stderr)
			}

			// Retry.
			logger.Pl.E("Command attempt %d failed for %q due to error: %v:\n\nCaptured output:\n%s", i, baseName, err, stderr)
			continue
		}
		// FFmpeg completed successfully: Exit loop.
		logger.Pl.S("FFmpeg successfully processed %q", baseName)
		break
	}
	return nil
}

// finalizeOutput moves the new video from tmpOutPath into place, backing up or removing the original as set.
func finalizeOutput(fd *models.FileData, tmpOutPath, outputFileExt string, replaced bool) (err error) {
	inputFilePath := fd.OriginalVideoPath

	// Back up the original if the new video takes its path, unless it's purged.
	samePath := inputFilePath == fd.PostFFmpegVideoPath
	purgeOriginal := abstractions.GetBool(keys.PurgeOriginalVideo)
	if samePath && (!purgeOriginal || abstractions.GetBool(keys.NoFileOverwrite)) {
		if fd.BackupVideoPath, err = makeBackup(inputFilePath); err != nil {
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
	report.VideoPath(fd.PostFFmpegVideoPath)
	if !samePath && !replaced && purgeOriginal {
		logger.Pl.I("Original file not type %s, removing %q", outputFileExt, inputFilePath)
		if err := os.Remove(inputFilePath); err != nil {
			vars.AddToErrorArray(err)
			return fmt.Errorf("failed to remove original file (%s). Error: %w", inputFilePath, err)
		}
	}

	// Log success.
	fmt.Fprintf(os.Stderr, "\n")
	logger.Pl.S("Successfully processed video:\n\nOriginal file: %s\nNew file: %s\n\nTitle: %s", inputFilePath,
		fd.PostFFmpegVideoPath,
		fd.MTitleDesc.Title)

	return nil
}

// tempOutputName returns the name of a video's temp output file, hashing a base name too long to fit the filesystem's limit.
func tempOutputName(fileBase, inputFileExt, outExt string) string {
	name := sharedconsts.TempTag + fileBase + inputFileExt + outExt
	if len(name) <= sharedconsts.MaxNameBytes {
		return name
	}
	sum := sha256.Sum256([]byte(fileBase))
	return sharedconsts.TempTag + hex.EncodeToString(sum[:8]) + inputFileExt + outExt
}

// skipProcessing determines whether the program should process this video (meta already exists, file extensions are unchanged, and codecs match).
func skipProcessing(fd *models.FileData, currentVCodec, currentACodec sharedmodels.FromCodec, desiredVCodec, desiredACodec, outExt string) (skipProcessing bool) {
	logger.Pl.I("Checking if processing should continue for file %q...", fd.OriginalVideoPath)

	// Write thumbnail.
	if parsing.ForceThumbnailWrite() && fd.EmbedThumbnail() != "" {
		logger.Pl.I("Thumbnail detected. Will write to file.")
		return false
	}

	var (
		differentExt, codecsDiffer bool
	)

	// Check for extension difference.
	currentExt := strings.ToLower(filepath.Ext(fd.OriginalVideoPath))
	if !strings.EqualFold(currentExt, outExt) && outExt != "" {
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
		logger.Pl.I("For file %q, no processing is required. Metadata exists, desired codecs (including compression-level preferences) match, and the file extension matches. Skipping processing...", fd.OriginalVideoPath)

		// Save 'post-FFmpeg' video path into model.
		fd.PostFFmpegVideoPath = fd.OriginalVideoPath

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

// getCodecs checks the input codec to determine if a straight remux is possible.
func getCodecs(ctx context.Context, inputFile string) (videoCodec, audioCodec sharedmodels.FromCodec, err error) {
	blankFromCodec := sharedmodels.NewFromCodec(sharedconsts.CodecCopy, "")
	if inputFile == "" {
		return blankFromCodec, blankFromCodec, fmt.Errorf("input file is empty, cannot check codecs")
	}

	// Measuring a missing audio bitrate reads the whole file, so only do it when a codec line goes by compression level.
	measureAudio := mapsCompressionLevels(keys.TranscodeVideoCodecMap) || mapsCompressionLevels(keys.TranscodeAudioCodecMap)

	// Determine codecs and video compression level.
	if audioCodec.Codec, videoCodec.Codec, audioCodec.Compression, videoCodec.Compression, err = ffprobe.ProbeCodecs(ctx, inputFile, measureAudio); err != nil {
		return blankFromCodec, blankFromCodec, err
	}

	// Use the names codec mappings do, as FFprobe's can differ (e.g. "mpeg2video", "pcm_s16le"). Unknown ones stay as they are.
	if v, err := sharedvalidation.ValidateVideoCodec(videoCodec.Codec); err == nil && v != "" {
		videoCodec.Codec = v
	}
	if a, err := sharedvalidation.ValidateAudioCodec(audioCodec.Codec); err == nil && a != "" {
		audioCodec.Codec = a
	}

	// Log and return detected codecs.
	logger.Pl.D(1, "Detected codecs - video: %s, audio: %s", videoCodec.Codec, audioCodec.Codec)
	return videoCodec, audioCodec, nil
}
