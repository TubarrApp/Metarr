package metadata

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/models"
	backup "Metarr/internal/utils/fs/backup"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecuteVideo writes metadata to a single video file
func ExecuteVideo(fd *models.FileData) error {

	if dontProcess(fd) {
		return nil
	}

	var (
		tmpOutPath,
		outExt string
	)

	dir := fd.VideoDirectory
	origPath := fd.OriginalVideoPath
	origExt := filepath.Ext(origPath)

	if outExt = config.GetString(keys.OutputFiletype); outExt == "" {
		outExt = origExt
		config.Set(keys.OutputFiletype, outExt)
	} else {
		outExt = origExt
	}

	fmt.Printf("\nWriting metadata for file: %s\n", origPath)

	// Make temp output path
	fileBase := strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))
	tmpOutPath = filepath.Join(dir, consts.TempTag+fileBase+origExt+outExt)
	logging.PrintD(3, "Orig ext: '%s', Out ext: '%s'", origExt, outExt)

	// Add temp path to data struct
	fd.TempOutputFilePath = tmpOutPath

	defer func() {
		if _, err := os.Stat(tmpOutPath); err == nil {
			os.Remove(tmpOutPath)
		}
	}()

	// Build FFmpeg command
	builder := newFfCommandBuilder(fd, tmpOutPath)
	args, err := builder.buildCommand(fd, outExt)
	if err != nil {
		return err
	}

	command := exec.Command("ffmpeg", args...)
	logging.PrintI("%sConstructed FFmpeg command for%s '%s':\n\n%v\n", consts.ColorCyan, consts.ColorReset, fd.OriginalVideoPath, command.String())

	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	// Set final video path and base name in model
	fd.FinalVideoBaseName = strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))
	fd.FinalVideoPath = filepath.Join(fd.VideoDirectory, fd.FinalVideoBaseName) + outExt

	logging.PrintI("Video file path data:\n\nOriginal Video Path: %s\nMetadata File Path: %s\nFinal Video Path: %s\n\nTemp Output Path: %s", origPath,
		fd.JSONFilePath,
		fd.FinalVideoPath,
		fd.TempOutputFilePath)

	// Run the ffmpeg command
	logging.Print("%s!!! Starting FFmpeg command for '%s'...\n%s", consts.ColorCyan, fd.FinalVideoBaseName, consts.ColorReset)
	if err := command.Run(); err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return fmt.Errorf("failed to run FFmpeg command: %w", err)
	}

	// Rename temporary file to overwrite the original video file
	if filepath.Ext(origPath) != filepath.Ext(fd.FinalVideoPath) {
		logging.PrintI("Original file not type %s, removing '%s'", outExt, origPath)

	} else if config.GetBool(keys.NoFileOverwrite) && origPath == fd.FinalVideoPath {
		if err := makeBackup(origPath); err != nil {
			return err
		}
	}

	// Delete original after potential backup ops
	err = os.Remove(origPath)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return fmt.Errorf("failed to remove original file (%s). Error: %v", origPath, err)
	}

	//
	err = os.Rename(tmpOutPath, fd.FinalVideoPath)
	if err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	logging.PrintS(0, "Successfully processed video:\n\nOriginal file: %s\nNew file: %s\n\nTitle: %s", origPath,
		fd.FinalVideoPath,
		fd.MTitleDesc.Title)

	return nil
}

// dontProcess determines whether the program should process this video (meta already exists and file extensions are unchanged)
func dontProcess(fd *models.FileData) (dontProcess bool) {
	if fd.MetaAlreadyExists {

		logging.PrintI("Metadata already exists in the file, skipping processing...")
		origPath := fd.OriginalVideoPath
		fd.FinalVideoBaseName = strings.TrimSuffix(filepath.Base(origPath), filepath.Ext(origPath))

		// Set the final video path based on output extension
		outExt := config.GetString(keys.OutputFiletype)
		if outExt == "" {
			outExt = filepath.Ext(fd.OriginalVideoPath)
			config.Set(keys.OutputFiletype, outExt)
		}

		fd.FinalVideoPath = filepath.Join(fd.VideoDirectory, fd.FinalVideoBaseName) + outExt
		return true
	}
	return dontProcess
}

// makeBackup performs the backup
func makeBackup(origPath string) error {

	origInfo, err := os.Stat(origPath)
	if os.IsNotExist(err) {
		logging.PrintI("File does not exist, safe to proceed overwriting: %s", origPath)
		return nil
	}

	backupPath, err := backup.RenameToBackup(origPath)
	if err != nil {
		return fmt.Errorf("failed to rename original file and preserve file is on, aborting: %w", err)
	}

	backInfo, err := os.Stat(backupPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("backup file was not created, aborting")
	}

	if origInfo.Size() != backInfo.Size() {
		return fmt.Errorf("backup file size does not match original, aborting")
	}

	return nil
}
