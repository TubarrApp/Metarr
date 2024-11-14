package metadata

import (
	"context"
	"fmt"
	"metarr/internal/cfg"
	"metarr/internal/dates"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	process "metarr/internal/metadata/process/json"
	check "metarr/internal/metadata/reader/check_existing"
	tags "metarr/internal/metadata/tags"
	jsonRw "metarr/internal/metadata/writer/json"
	"metarr/internal/models"
	"metarr/internal/transformations"
	logging "metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu sync.Mutex
)

// ProcessJSONFile reads a single JSON file and fills in the metadata
func ProcessJSONFile(ctx context.Context, fd *models.FileData) (*models.FileData, error) {
	if fd == nil {
		return nil, fmt.Errorf("model passed in null")
	}

	logging.D(2, "Beginning JSON file processing...")

	// Function mutex
	mu.Lock()
	defer mu.Unlock()

	filePath := fd.JSONFilePath

	// Open the file
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		logging.ErrorArray = append(logging.ErrorArray, err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Grab and store metadata reader/writer
	jsonRW := jsonRw.NewJSONFileRW(file)
	if jsonRW != nil {
		fd.JSONFileRW = jsonRW
	}

	// Decode metadata from file
	data, err := fd.JSONFileRW.DecodeJSON(file)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, fmt.Errorf("json decoded nil")
	}

	logging.D(3, "%v", data)

	var ok bool

	// Get web data first (before MakeMetaEdits in case of transformation presets)
	if ok = process.FillWebpageDetails(fd, data); ok {
		logging.I("URLs grabbed: %s", fd.MWebData.TryURLs)
	}

	if len(fd.MWebData.TryURLs) > 0 {
		if match := transformations.TryTransPresets(fd.MWebData.TryURLs, fd); match == "" {
			logging.D(1, "No presets found for video '%s' URLs %v", fd.OriginalVideoBaseName, fd.MWebData.TryURLs)
		}
	}

	// Make metadata adjustments per user selection or transformation preset
	if edited, err := fd.JSONFileRW.MakeJSONEdits(file, fd); err != nil {
		return nil, err
	} else if edited {
		logging.D(2, "Refreshing JSON metadata after edits were made...")
		if data, err = fd.JSONFileRW.RefreshJSON(); err != nil {
			return nil, err
		}
	}

	// Fill timestamps and make/delete date tag ammendments
	if ok = process.FillTimestamps(fd, data); !ok {
		logging.I("No date metadata found")
	}

	if fd.MDates.FormattedDate == "" {
		dates.FormatAllDates(fd)
	}

	if cfg.IsSet(keys.MDateTagMap) || cfg.IsSet(keys.MDelDateTagMap) {
		ok, err = jsonRW.JSONDateTagEdits(file, fd)
		if err != nil {
			logging.E(0, err.Error())
		} else if !ok {
			logging.D(1, "Did not make date tag edits for metadata, tag already exists?")
		}
	} else {
		logging.D(4, "Skipping making metadata date tag edits, key not set")
	}

	// Must refresh JSON again after further edits
	data, err = jsonRW.RefreshJSON()
	if err != nil {
		return nil, err
	}

	// Fill other metafields
	if data, ok = process.FillJSONFields(fd, data); !ok {
		logging.D(2, "Some metafields were unfilled")
	}

	// Make filename date tag
	logging.D(3, "About to make date tag for: %v", file.Name())

	if cfg.IsSet(keys.FileDateFmt) {
		dateFmt, ok := cfg.Get(keys.FileDateFmt).(enums.DateFormat)

		switch {
		case !ok:
			logging.E(0, "Got null or wrong type for file date format. Got type %T", dateFmt)

		case dateFmt != enums.DATEFMT_SKIP:

			dateTag, err := tags.MakeDateTag(data, fd, dateFmt)
			if err != nil {
				logging.E(0, "Failed to make date tag: %v", err)
			}
			if !strings.Contains(file.Name(), dateTag) {
				fd.FilenameDateTag = dateTag
			}

		default:
			logging.D(1, "Set file date tag format to skip, not making date tag for '%s'", file.Name())
		}
	}

	// Add new filename tag for files
	if cfg.IsSet(keys.MFilenamePfx) {
		logging.D(3, "About to make prefix tag for: %v", file.Name())
		fd.FilenameMetaPrefix = tags.MakeFilenameTag(data, file)
	}

	// Check if metadata is already existent in target file
	if filetypeMetaCheckSwitch(ctx, fd) {
		logging.I("Metadata already exists in target file '%s', will skip processing", fd.OriginalVideoBaseName)
		fd.MetaAlreadyExists = true
	}

	return fd, nil
}

func filetypeMetaCheckSwitch(ctx context.Context, fd *models.FileData) bool {

	logging.D(4, "Entering filetypeMetaCheckSwitch with '%s'", fd.OriginalVideoPath)

	var outExt string
	outFlagSet := cfg.IsSet(keys.OutputFiletype)

	if outFlagSet {
		outExt = cfg.GetString(keys.OutputFiletype)
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
		logging.I("Input format '%s' differs from output format '%s', will not run metadata checks", currentExt, outExt)
		return false
	}

	// Run metadata checks in all other cases
	switch currentExt {
	case consts.ExtMP4:
		return check.MP4MetaMatches(ctx, fd)
	default:
		logging.I("Checks not currently implemented for this filetype")
		return false
	}
}
