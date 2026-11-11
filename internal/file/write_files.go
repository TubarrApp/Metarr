package file

import (
	"context"
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/parsing"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/TubarrApp/gocommon/logging"
	"github.com/TubarrApp/gocommon/sharedconsts"
)

// posterSuffix ends the name of a video's thumbnail sidecar, which media servers read beside the video.
const posterSuffix = "-poster"

// posterExts maps image content types to the thumbnail sidecar extensions Metarr writes and moves.
var posterExts = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// FSFileWriter is a model granting access to file writer functions.
type FSFileWriter struct {
	Fd           *models.FileData
	SkipVids     bool
	RenamedVideo string
	InputVideo   string
	RenamedMeta  string
	InputMeta    string
	OutputDir    string
	muFs         sync.RWMutex
}

// NewFSFileWriter returns a file writer, used for writing changes to filenames etc.
func NewFSFileWriter(fd *models.FileData, skipVids bool, outputDir string) (*FSFileWriter, error) {
	inputVid := fd.PostFFmpegVideoPath
	renamedVid := fd.RenamedVideoPath
	inputMeta := fd.MetaFilePath
	renamedMeta := fd.RenamedMetaPath

	if !skipVids {
		if inputVid == "" && renamedVid == "" {
			return nil, fmt.Errorf("some required video paths are empty:\n\nVid src: %q\nVid dest: %q", inputVid, renamedVid)
		}
	}

	// A video can have no metafile, so only a metadata-only run needs the meta paths.
	if skipVids && inputMeta == "" && renamedMeta == "" {
		return nil, fmt.Errorf("some required meta paths are empty:\n\nMeta src: %q\nMeta dest: %q", inputMeta, renamedMeta)
	}

	if logging.Level() > 1 {
		differ := 0
		if !strings.EqualFold(renamedVid, inputVid) {
			differ++
		}
		if !strings.EqualFold(renamedMeta, inputMeta) {
			differ++
		}

		logger.Pl.D(3, "Made FSFileWriter with parameters:\n\nSkip videos? %v\n\nOriginal Video: %s\nRenamed Video:  %s\n\nOriginal Metafile: %s\nRenamed Metafile:  %s\n\n%d file names will be changed...\n\n",
			skipVids, inputVid, renamedVid, inputMeta, renamedMeta, differ)
	}

	return &FSFileWriter{
		Fd:           fd,
		SkipVids:     skipVids,
		RenamedVideo: renamedVid,
		InputVideo:   inputVid,
		RenamedMeta:  renamedMeta,
		InputMeta:    inputMeta,
		OutputDir:    outputDir,
	}, nil
}

// RenameAndMoveFiles moves the video/meta files to their new paths, in place or in the output directory, leaving out the meta
// file if noMeta (i.e. it was purged). The video's NFO file goes with it.
func (fs *FSFileWriter) RenameAndMoveFiles(noMeta bool) error {
	fs.muFs.Lock()
	defer fs.muFs.Unlock()

	if fs.RenamedVideo == "" && fs.RenamedMeta == "" {
		logger.Pl.D(1, "Skipping video or metadata renaming, as the renamed strings are empty")
		return nil
	}

	// Create the output directory if one is set.
	if fs.OutputDir != "" {
		if err := os.MkdirAll(fs.OutputDir, consts.PermsVideoDir); err != nil {
			return fmt.Errorf("failed to create or find destination directory: %w", err)
		}
	}

	// Move video file.
	moveVideo := shouldProcess(fs.InputVideo, fs.RenamedVideo, true, fs.SkipVids)
	if moveVideo {
		if err := moveOrCopyFile(fs.InputVideo, fs.RenamedVideo); err != nil {
			return fmt.Errorf("failed to move video file from %q → %q: %w", fs.InputVideo, fs.RenamedVideo, err)
		}
	}

	// Move the video's thumbnail sidecars, whose names no metafile can need.
	if moveVideo {
		fs.moveVideoPosters()
	}

	// Move meta file. A failure still lets the video's NFO follow the video below.
	var metaErr error
	if !noMeta && shouldProcess(fs.InputMeta, fs.RenamedMeta, false, fs.SkipVids) {
		if err := moveOrCopyFile(fs.InputMeta, fs.RenamedMeta); err != nil {
			metaErr = fmt.Errorf("failed to move metadata file from %q → %q: %w", fs.InputMeta, fs.RenamedMeta, err)
		}
	}

	// Move the video's NFO file, after the metafile so it can't take a name the metafile needs.
	if moveVideo {
		fs.moveVideoNFO()
	}
	return metaErr
}

// moveVideoPosters moves the thumbnail sidecars named for the input video to the renamed video's name, only warning on failure.
func (fs *FSFileWriter) moveVideoPosters() {
	for _, ext := range posterExts {
		src := posterPath(fs.InputVideo, ext)
		if info, err := os.Stat(src); err != nil || !info.Mode().IsRegular() {
			continue
		}
		if err := moveOrCopyFile(src, posterPath(fs.RenamedVideo, ext)); err != nil {
			logger.Pl.W("Failed to move the video's thumbnail sidecar: %v", err)
		}
	}
}

// posterPath returns the path of a video's thumbnail sidecar with the given extension.
func posterPath(videoPath, ext string) string {
	return strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + posterSuffix + ext
}

// UpdateThumbnailSidecars saves --thumbnail beside each final video as "<video name>-poster.<ext>", for media servers,
// or removes the sidecars if --strip-thumbnail is set. A URL only gets a sidecar for a video which can't embed it.
func UpdateThumbnailSidecars(ctx context.Context, fdArray []*models.FileData) {
	if abstractions.GetBool(keys.StripThumbnails) {
		for _, fd := range fdArray {
			if fd != nil && fd.FinalVideoPath != "" {
				removePosters(fd.FinalVideoPath, "")
			}
		}
		return
	}

	thumbnail := abstractions.GetString(keys.Thumbnail)
	if thumbnail == "" {
		return
	}
	isURL := parsing.IsHTTPURL(thumbnail)

	var (
		data []byte
		ext  string
		err  error
	)
	for _, fd := range fdArray {
		if fd == nil || fd.FinalVideoPath == "" {
			continue
		}
		// The NFO holds a URL and the video embeds it, so no sidecar is needed.
		if isURL && parsing.HoldsThumbnail(filepath.Ext(fd.FinalVideoPath)) {
			continue
		}
		// Read the thumbnail once, on the first video needing it.
		if data == nil {
			if data, ext, err = readPoster(ctx, thumbnail, isURL); err != nil {
				logger.Pl.E("Not saving thumbnail %q beside the video: %v", thumbnail, err)
				return
			}
		}
		if err := writePoster(fd.FinalVideoPath, ext, data); err != nil {
			logger.Pl.E("Failed to save thumbnail %q beside %q: %v", thumbnail, fd.FinalVideoPath, err)
		}
	}
}

// readPoster returns a thumbnail's image and its sidecar extension, downloading it first if it's a URL.
func readPoster(ctx context.Context, thumbnail string, isURL bool) (data []byte, ext string, err error) {
	path := thumbnail
	if isURL {
		if path, err = DownloadThumbnail(ctx, thumbnail, "sidecar"); err != nil {
			return nil, "", fmt.Errorf("failed to download it: %w", err)
		}
		defer func() {
			if err := os.Remove(path); err != nil {
				logger.Pl.W("Failed to remove downloaded thumbnail %q: %v", path, err)
			}
		}()
	}
	if data, err = os.ReadFile(path); err != nil {
		return nil, "", fmt.Errorf("failed to read it: %w", err)
	}

	// Name the sidecar by what the file holds, as its own name may not say.
	ext, ok := posterExts[http.DetectContentType(data)]
	if !ok {
		return nil, "", errors.New("it isn't a JPG, PNG or WebP image")
	}
	return data, ext, nil
}

// writePoster writes a video's thumbnail sidecar, removing any it has with another extension.
func writePoster(videoPath, ext string, data []byte) error {
	// Leave existing sidecars alone if overwriting is disabled.
	if abstractions.GetBool(keys.NoFileOverwrite) {
		for _, e := range posterExts {
			if _, err := os.Stat(posterPath(videoPath, e)); err == nil {
				logger.Pl.I("Thumbnail sidecar %q already exists and overwriting is disabled, skipping", posterPath(videoPath, e))
				return nil
			}
		}
	}

	dst := posterPath(videoPath, ext)
	if err := os.WriteFile(dst, data, consts.PermsImageFile); err != nil {
		return fmt.Errorf("failed to write thumbnail sidecar %q: %w", dst, err)
	}

	// Remove sidecars with another extension, so media servers only find the new one.
	removePosters(videoPath, ext)
	logger.Pl.S("Saved thumbnail sidecar %q", dst)
	return nil
}

// removePosters deletes a video's thumbnail sidecars, except the one with keepExt, only warning on failure.
func removePosters(videoPath, keepExt string) {
	for _, e := range posterExts {
		if e == keepExt {
			continue
		}
		path := posterPath(videoPath, e)
		err := os.Remove(path)
		switch {
		case err == nil:
			logger.Pl.I("Removed thumbnail sidecar %q", path)
		case !errors.Is(err, os.ErrNotExist):
			logger.Pl.W("Failed to remove thumbnail sidecar %q: %v", path, err)
		}
	}
}

// moveVideoNFO moves the NFO file named for the input video to the renamed video's name. An NFO metafile is left to the
// metafile's own move or purge, and a failure only warns, as the video has already moved.
func (fs *FSFileWriter) moveVideoNFO() {
	src := nfoPath(fs.InputVideo)
	info, err := os.Stat(src)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	if metaInfo, err := os.Stat(fs.InputMeta); err == nil && os.SameFile(info, metaInfo) {
		return
	}
	if err := moveOrCopyFile(src, nfoPath(fs.RenamedVideo)); err != nil {
		logger.Pl.W("Failed to move the video's NFO file: %v", err)
	}
}

// nfoPath returns the path of the NFO file named for a video.
func nfoPath(videoPath string) string {
	return strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + sharedconsts.MExtNFO
}

// PurgesMetafile reports whether the metafile purge setting covers this file.
func (fs *FSFileWriter) PurgesMetafile(file string) (bool, error) {
	if !abstractions.IsSet(keys.MetaPurgeEnum) {
		return false, errors.New("meta purge enum not set")
	}

	e, ok := abstractions.Get(keys.MetaPurgeEnum).(enums.PurgeMetafiles)
	if !ok {
		return false, fmt.Errorf("wrong type for purge metafile enum. Got %T", e)
	}

	ext := filepath.Ext(file)
	ext = strings.ToLower(ext)

	switch e {
	case enums.PurgeMetaAll:
		return true, nil

	case enums.PurgeMetaJSON:
		if ext != sharedconsts.MExtJSON {
			logger.Pl.D(3, "Skipping deletion of metafile %q as extension does not match user selection", file)
			return false, nil
		}
		return true, nil

	case enums.PurgeMetaNFO:
		if ext != sharedconsts.MExtNFO {
			logger.Pl.D(3, "Skipping deletion of metafile %q as extension does not match user selection", file)
			return false, nil
		}
		return true, nil

	case enums.PurgeMetaNone: // A blank purge setting.
		return false, nil

	default:
		return false, errors.New("support not added for this metafile purge enum yet, exiting function")
	}
}

// DeleteMetafile safely removes metadata files once file operations are complete.
func (fs *FSFileWriter) DeleteMetafile(file string) (deleted bool, err error) {
	logger.Pl.I("Attempting to delete metafile %q...", file)

	if purges, err := fs.PurgesMetafile(file); err != nil || !purges {
		return false, err
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		return false, err
	}

	if fileInfo.IsDir() {
		return false, fmt.Errorf("metafile %q is a directory, not a file", file)
	}

	if !fileInfo.Mode().IsRegular() {
		return false, fmt.Errorf("metafile %q is not a regular file", file)
	}

	if err := os.Remove(file); err != nil {
		return false, fmt.Errorf("unable to delete meta file: %w", err)
	}
	logger.Pl.S("Successfully deleted metafile %q", file)

	return true, nil
}
