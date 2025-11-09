package processing

import (
	"context"
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/dates"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/ffprobe"
	"metarr/internal/metadata/fieldsjson"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
	"metarr/internal/transformations"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var jsonEditMutexMap sync.Map

// processJSONFile opens and processes a JSON file.
func processJSONFile(ctx context.Context, fd *models.FileData) error {
	if fd == nil {
		return errors.New("model passed in null")
	}
	logging.D(2, "Beginning JSON file processing...")

	filePath := fd.MetaFilePath
	value, _ := jsonEditMutexMap.LoadOrStore(filePath, &sync.Mutex{})
	fileMutex, ok := value.(*sync.Mutex)
	if !ok {
		return fmt.Errorf("internal error: mutex map corrupted for file %s", filePath)
	}
	fileMutex.Lock()
	defer fileMutex.Unlock()

	// Open the file
	file, err := os.OpenFile(filePath, os.O_RDWR, 0o644)
	if err != nil {
		logging.AddToErrorArray(err)
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logging.E("Failed to close file %q: %v", file.Name(), closeErr)
		}
	}()

	// Grab and store metadata reader/writer
	jsonRW := metawriters.NewJSONFileRW(ctx, file)

	// Decode metadata from file
	data, err := jsonRW.DecodeJSON(file)
	if err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("json decoded nil for file %q", file.Name())
	}

	// Get web data first (before MakeMetaEdits in case of transformation presets)
	gotWebData := fieldsjson.FillWebpageDetails(fd, data)
	if gotWebData {
		logging.I("URLs grabbed: %s", fd.MWebData.TryURLs)
	}
	if len(fd.MWebData.TryURLs) > 0 {
		if match := transformations.TryTransPresets(fd.MWebData.TryURLs, fd); match == "" {
			logging.D(1, "No presets found for video %q URLs %v", fd.OriginalVideoBaseName, fd.MWebData.TryURLs)
		}
	}

	// Make metadata adjustments per user selection or transformation preset
	if edited, err := jsonRW.MakeJSONEdits(file, fd); err != nil {
		return err
	} else if edited {
		logging.D(2, "Refreshing JSON metadata after edits were made...")
		if data, err = jsonRW.RefreshJSON(); err != nil {
			return err
		}
	}

	// Fill timestamps and make/delete date tag amendments
	if ok = fieldsjson.FillTimestamps(fd, data, jsonRW); !ok {
		logging.I("No date metadata found")
	}
	if fd.MDates.FormattedDate == "" {
		dates.FormatAllDates(fd)
	}

	// Make date tag edits if meta ops requires it
	if len(fd.MetaOps.DateTags) > 0 || len(fd.MetaOps.DeleteDateTags) > 0 {
		ok, err = jsonRW.JSONDateTagEdits(file, fd)
		if err != nil {
			logging.E("Failed to make date tag edits for metadata in file %q: %v", file.Name(), err)
		} else if !ok {
			logging.D(1, "Did not make date tag edits for metadata, tag already exists?")
		}
	} else {
		logging.D(4, "Skipping making metadata date tag edits, key not set")
	}

	// Must refresh JSON again after further edits
	data, err = jsonRW.RefreshJSON()
	if err != nil {
		return err
	}

	// Fill other metafields
	if data, ok = fieldsjson.FillJSONFields(fd, data, jsonRW); !ok {
		logging.D(2, "Some metafields were unfilled")
	}

	// Construct date tag:
	logging.D(1, "About to make date tag for: %v", file.Name())

	// Make date tag and apply to model if necessary
	if fd.FilenameOps != nil && fd.FilenameOps.DateTag.DateFormat != enums.DateFmtSkip {
		if dateTag, err := dates.MakeDateTag(data, fd, fd.FilenameOps.DateTag.DateFormat); err != nil {
			logging.E("Failed to make date tag: %v", err)
		} else if strings.Contains(file.Name(), dateTag) {
			logging.I("Date tag %q already found in file name %q", dateTag, file.Name())
		} else {
			fd.FilenameDateTag = dateTag
		}
	}

	// Check if metadata is already existent in target file
	if filetypeMetaCheckSwitch(ctx, fd) {
		logging.I("Metadata already exists in target file %q", fd.OriginalVideoBaseName)
		fd.MetaAlreadyExists = true
	}
	return nil
}

// filetypeMetaCheckSwitch checks metadata matches by file extension (different extensions store different fields).
func filetypeMetaCheckSwitch(ctx context.Context, fd *models.FileData) bool {
	logging.D(4, "Entering filetypeMetaCheckSwitch with %q", fd.OriginalVideoPath)

	var outExt string
	outFlagSet := abstractions.IsSet(keys.OutputFiletype)

	if outFlagSet {
		outExt = abstractions.GetString(keys.OutputFiletype)
	} else {
		outExt = filepath.Ext(fd.OriginalVideoPath)
		logging.D(2, "Got output extension as %s", outExt)
	}

	currentExt := filepath.Ext(fd.OriginalVideoPath)
	currentExt = strings.TrimSpace(currentExt)

	if outExt != "" && !strings.HasPrefix(outExt, ".") {
		outExt = "." + outExt

		logging.D(2, "Added dot to outExt: %s, currentExt is %s", outExt, currentExt)
	}

	if outFlagSet && outExt != "" && !strings.EqualFold(outExt, currentExt) {
		logging.I("Input format %q differs from output format %q, will not run metadata checks", currentExt, outExt)
		return false
	}

	// Write thumbnail
	if abstractions.IsSet(keys.ForceWriteThumbnails) {
		if abstractions.GetBool(keys.ForceWriteThumbnails) && fd.MWebData.Thumbnail != "" {
			logging.I("Skipping FFprobe, thumbnail enforcing write.")
			return false
		}
	}

	// Run metadata checks in all other cases
	switch currentExt {
	case consts.ExtMP4:
		return ffprobe.MP4MetaMatches(ctx, fd)
	default:
		logging.I("Checks not currently implemented for this filetype")
		return false
	}
}
