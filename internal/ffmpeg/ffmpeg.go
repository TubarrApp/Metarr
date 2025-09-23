// Package ffmpeg handles FFmpeg command building and execution.
package ffmpeg

import (
	"context"
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/fs/backup"
	"metarr/internal/utils/logging"
	"metarr/internal/utils/validation"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecuteVideo writes metadata to a single video file.
func ExecuteVideo(ctx context.Context, fd *models.FileData) error {
	var (
		tmpOutPath, outExt string
	)

	origPath := fd.OriginalVideoPath
	origExt := filepath.Ext(origPath)

	// Extension validation - now checks length and format immediately
	if cfg.IsSet(keys.OutputFiletype) {
		if outExt = validation.ValidateExtension(cfg.GetString(keys.OutputFiletype)); outExt == "" {
			logging.E(0, "Grabbed output extension but extension was empty/invalid, reverting to original: %s", origExt)
			outExt = origExt
		}
	} else {
		outExt = origExt
	}

	if skipProcessing(fd, outExt) {
		return nil
	}

	logging.I("Will execute video from extension %q → %q", origExt, outExt)

	fmt.Printf("\nWriting metadata for file: %s\n", origPath)

	dir := fd.VideoDirectory
	fileBase := strings.TrimSuffix(filepath.Base(origPath), origExt)

	// Make temp output path
	tmpOutPath = filepath.Join(dir, consts.TempTag+fileBase+origExt+outExt)
	logging.D(3, "Orig ext: %q, Out ext: %q", origExt, outExt)

	// Add temp path to data struct
	fd.TempOutputFilePath = tmpOutPath

	defer func() {
		if _, err := os.Stat(tmpOutPath); err == nil {
			if err := os.Remove(tmpOutPath); err != nil {
				logging.E(0, "Failed to remove %q: %v", tmpOutPath, err)
			}
		}
	}()

	// Build FFmpeg command
	builder := newFfCommandBuilder(fd, tmpOutPath)
	args, err := builder.buildCommand(fd, outExt)
	if err != nil {
		return err
	}

	command := exec.CommandContext(ctx, "ffmpeg", args...)
	logging.I("%sConstructed FFmpeg command for%s %q:\n\n%v\n", consts.ColorCyan, consts.ColorReset, fd.OriginalVideoPath, command.String())

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	// Set final video path and base name in model
	fd.FinalVideoBaseName = strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))
	fd.FinalVideoPath = filepath.Join(fd.VideoDirectory, fd.FinalVideoBaseName) + outExt

	logging.I("Video file path data:\n\nOriginal Video Path: %s\nMetadata File Path: %s\nFinal Video Path: %s\n\nTemp Output Path: %s", origPath,
		fd.JSONFilePath,
		fd.FinalVideoPath,
		fd.TempOutputFilePath)

	// Run the ffmpeg command
	logging.P("%s!!! Starting FFmpeg command for %q...\n%s", consts.ColorCyan, fd.FinalVideoBaseName, consts.ColorReset)
	if err := command.Run(); err != nil {
		logging.AddToErrorArray(err)
		return fmt.Errorf("failed to run FFmpeg command: %w", err)
	}

	// Rename temporary file to overwrite the original video file
	if filepath.Ext(origPath) != filepath.Ext(fd.FinalVideoPath) {
		logging.I("Original file not type %s, removing %q", outExt, origPath)

	} else if cfg.GetBool(keys.NoFileOverwrite) && origPath == fd.FinalVideoPath {
		if err := makeBackup(origPath); err != nil {
			return err
		}
	}

	// Delete original after potential backup ops
	err = os.Remove(origPath)
	if err != nil {
		logging.AddToErrorArray(err)
		return fmt.Errorf("failed to remove original file (%s). Error: %w", origPath, err)
	}

	//
	err = os.Rename(tmpOutPath, fd.FinalVideoPath)
	if err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	logging.S(0, "Successfully processed video:\n\nOriginal file: %s\nNew file: %s\n\nTitle: %s", origPath,
		fd.FinalVideoPath,
		fd.MTitleDesc.Title)

	return nil
}

// skipProcessing determines whether the program should process this video (meta already exists, file extensions are unchanged, and codecs match).
func skipProcessing(fd *models.FileData, outExt string) bool {

	logging.I("Checking if processing should continue for file %q...", fd.OriginalVideoBaseName)

	var (
		desiredVCodec, desiredACodec           string
		differentExt, metaExists, codecsDiffer bool
	)

	// Check for extension difference
	currentExt := strings.ToLower(filepath.Ext(fd.OriginalVideoPath))

	if currentExt != outExt {
		differentExt = true
	}

	logging.D(2, "Current extension: %q\nDesired extension: %q\n\nExtensions differ? %v", currentExt, outExt, differentExt)

	// Check codec mismatches
	if cfg.IsSet(keys.TranscodeCodec) {
		desiredVCodec = cfg.GetString(keys.TranscodeCodec)
	}
	if cfg.IsSet(keys.TranscodeAudioCodec) {
		desiredACodec = cfg.GetString(keys.TranscodeAudioCodec)
	}

	if desiredVCodec != "" || desiredACodec != "" {
		vCodec, aCodec, err := checkCodecs(fd.OriginalVideoPath)
		if err != nil {
			logging.E(0, "Failed to check input file %q codec: %v", fd.OriginalVideoBaseName, err)
		}

		if desiredVCodec != vCodec || desiredACodec != aCodec {
			codecsDiffer = true
		}
		logging.D(2, "Codec check for %q:\n\nCurrent video codecs:\n\nVideo: %q\nAudio: %q\n\nDesired video codecs:\n\nVideo: %q\nAudio: %q\n\nCodecs differ? %v", fd.OriginalVideoBaseName, vCodec, aCodec, desiredVCodec, desiredACodec, codecsDiffer)
	}

	// Check if metadata already exists
	metaExists = fd.MetaAlreadyExists
	logging.D(2, "Metadata mismatch in file %q", fd.OriginalVideoBaseName)

	// Final checks
	if !codecsDiffer && !differentExt && metaExists {

		logging.I("For file %q, all metadata exists, codecs match, and extensions match. Skipping processing...", fd.OriginalVideoBaseName)
		origPath := fd.OriginalVideoPath
		fd.FinalVideoBaseName = strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))

		// Save final video path into model
		fd.FinalVideoPath = filepath.Join(fd.VideoDirectory, fd.FinalVideoBaseName) + outExt
		return true
	}

	logging.I("Metadata, codec, or file extension mismatch. Continuing to process file %q", fd.OriginalVideoBaseName)
	return false
}

// makeBackup performs the backup.
func makeBackup(origPath string) error {

	origInfo, err := os.Stat(origPath)
	if os.IsNotExist(err) {
		logging.I("File does not exist, safe to proceed overwriting: %s", origPath)
		return nil
	}

	backupPath, err := backup.RenameToBackup(origPath)
	if err != nil {
		return fmt.Errorf("failed to rename original file and preserve file is on, aborting: %w", err)
	}

	backInfo, err := os.Stat(backupPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("backup file %q was not created, aborting", backupPath)
	}

	if origInfo.Size() != backInfo.Size() {
		return fmt.Errorf("backup file size %d does not match original %d, aborting", origInfo.Size(), backInfo.Size())
	}

	return nil
}

// checkCodecs checks the input codec to determine if a straight remux is possible.
func checkCodecs(inputFile string) (videoCodec, audioCodec string, err error) {

	if inputFile == "" {
		return "", "", fmt.Errorf("input file is empty, cannot check codecs")
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputFile,
	)

	videoCodecBytes, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("cannot read video codec: %v", err)
	}
	videoCodec = strings.TrimSpace(string(videoCodecBytes))

	cmd = exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a:0", // first audio stream
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputFile,
	)

	audioCodecBytes, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("cannot read audio codec: %v", err)
	}
	audioCodec = strings.TrimSpace(string(audioCodecBytes))

	logging.D(1, "Detected codecs - video: %s, audio: %s", videoCodec, audioCodec)

	return videoCodec, audioCodec, nil
}
