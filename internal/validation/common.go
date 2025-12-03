// Package validation handles validation of user flag input.
package validation

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedvalidation"
	"github.com/shirou/gopsutil/mem"
)

// ValidateGPUAcceleration validates the user input GPU selection.
func ValidateGPUAcceleration(g string) (accelType string, err error) {
	if g, err = sharedvalidation.ValidateGPUAccelType(g); err != nil {
		return "", err
	}

	// Check OS compatibility with acceleration type.
	if !sharedvalidation.OSSupportsAccelType(g) {
		logger.Pl.W("GPU acceleration type %q is not supported on this operating system, omitting.", g)
		return "", nil
	}

	return g, nil
}

// ---- Validate And Set ------------------------------------------------------------------------------------------

// ValidateAndSetVideoCodec sets mappings for video codec inputs and transcode options.
func ValidateAndSetVideoCodec(pairs []string) error {
	vCodecMap := map[string]string{
		sharedconsts.VCodecAV1:   sharedconsts.VCodecCopy,
		sharedconsts.VCodecH264:  sharedconsts.VCodecCopy,
		sharedconsts.VCodecHEVC:  sharedconsts.VCodecCopy,
		sharedconsts.VCodecMPEG2: sharedconsts.VCodecCopy,
		sharedconsts.VCodecVP8:   sharedconsts.VCodecCopy,
		sharedconsts.VCodecVP9:   sharedconsts.VCodecCopy,
	}

	// Deduplicate.
	dedupPairs := make([]string, 0, len(pairs))
	for _, p := range pairs {
		if p == "" {
			continue
		}
		if !slices.Contains(dedupPairs, p) {
			dedupPairs = append(dedupPairs, p)
		}
	}

	// Iterate deduped pairs.
	for _, p := range dedupPairs {
		split := strings.Split(p, ":")
		if len(split) < 1 {
			return fmt.Errorf("impossible condition splitting %q", p)
		}
		input, err := sharedvalidation.ValidateVideoCodec(split[0]) // Safe (split returns non-empty 'p').
		if err != nil {
			return err
		}

		// Singular value, apply to every entry.
		if len(split) < 2 {
			for k := range vCodecMap {
				if input == k {
					continue
				}
				vCodecMap[k] = input
			}
			continue
		}

		// Multi value entry, apply specific output to specific input.
		output := split[1]
		output, err = sharedvalidation.ValidateVideoCodec(output)
		if err != nil {
			return err
		}
		vCodecMap[input] = output
	}

	abstractions.Set(keys.TranscodeVideoCodecMap, vCodecMap)
	return nil
}

// ValidateAndSetAudioCodec sets mappings for audio codec inputs and transcode options.
func ValidateAndSetAudioCodec(pairs []string) (err error) {
	aCodecMap := map[string]string{
		sharedconsts.ACodecAAC:    sharedconsts.ACodecCopy,
		sharedconsts.ACodecAC3:    sharedconsts.ACodecCopy,
		sharedconsts.ACodecALAC:   sharedconsts.ACodecCopy,
		sharedconsts.ACodecDTS:    sharedconsts.ACodecCopy,
		sharedconsts.ACodecEAC3:   sharedconsts.ACodecCopy,
		sharedconsts.ACodecFLAC:   sharedconsts.ACodecCopy,
		sharedconsts.ACodecMP2:    sharedconsts.ACodecCopy,
		sharedconsts.ACodecMP3:    sharedconsts.ACodecCopy,
		sharedconsts.ACodecOpus:   sharedconsts.ACodecCopy,
		sharedconsts.ACodecPCM:    sharedconsts.ACodecCopy,
		sharedconsts.ACodecTrueHD: sharedconsts.ACodecCopy,
		sharedconsts.ACodecVorbis: sharedconsts.ACodecCopy,
		sharedconsts.ACodecWAV:    sharedconsts.ACodecCopy,
	}

	// Deduplicate.
	dedupPairs := make([]string, 0, len(pairs))
	for _, p := range pairs {
		if p == "" {
			continue
		}
		if !slices.Contains(dedupPairs, p) {
			dedupPairs = append(dedupPairs, p)
		}
	}

	// Iterate deduped pairs.
	for _, p := range dedupPairs {
		split := strings.Split(p, ":")
		if len(split) < 1 {
			return fmt.Errorf("impossible condition splitting %q", p)
		}
		input, err := sharedvalidation.ValidateAudioCodec(split[0]) // Safe (split returns non-empty 'p').
		if err != nil {
			return err
		}

		// Singular value, apply to every entry.
		if len(split) < 2 {
			for k := range aCodecMap {
				if input == k {
					continue
				}
				aCodecMap[k] = input
			}
			continue
		}

		// Multi value entry, apply specific output to specific input.
		output := split[1]
		output, err = sharedvalidation.ValidateAudioCodec(output)
		if err != nil {
			return err
		}
		aCodecMap[input] = output
	}

	abstractions.Set(keys.TranscodeAudioCodecMap, aCodecMap)
	return nil
}

// ValidateAndSetBatchPairs retrieves valid files and directories from a batch pair entry.
func ValidateAndSetBatchPairs(batchPairs []string) error {
	var vDirs, vFiles, mDirs, mFiles []string
	for _, pair := range batchPairs {
		split := strings.SplitN(pair, ":", 2)
		if len(split) < 2 {
			logger.Pl.W("skipping invalid batch pair %q, should be 'video_dir_path:json_dir_path'")
			continue
		}
		video := split[0]
		meta := split[1]

		// Ensure no colons in names.
		if strings.Contains(video, ":") || strings.Contains(meta, ":") {
			return fmt.Errorf("cannot use pair %q\nDO NOT put colons in file or folder names (FFmpeg treats as protocol)", pair)
		}

		// Handle video part.
		vStat, err := os.Stat(video)
		if err != nil {
			return err
		}
		mStat, err := os.Stat(meta)
		if err != nil {
			return err
		}
		vIsDir := vStat.IsDir()
		mIsDir := mStat.IsDir()

		// Check for mismatch.
		if vIsDir && !mIsDir || !vIsDir && mIsDir {
			return fmt.Errorf("mismatch in batch entry types: video is dir? %v, meta is dir? %v", vIsDir, mIsDir)
		}

		// Add videos.
		if vIsDir {
			vDirs = append(vDirs, video)
		} else {
			vFiles = append(vFiles, video)
		}

		// Add meta.
		if mIsDir {
			mDirs = append(mDirs, meta)
		} else {
			mFiles = append(mFiles, meta)
		}
	}
	abstractions.Set(keys.BatchPairs, models.BatchPairs{
		VideoDirs:  vDirs,
		VideoFiles: vFiles,
		MetaDirs:   mDirs,
		MetaFiles:  mFiles,
	})
	return nil
}

// ValidateAndSetConcurrencyLimit checks and ensures correct concurrency limit input.
func ValidateAndSetConcurrencyLimit(c int) int {
	c = sharedvalidation.ValidateConcurrencyLimit(c)
	abstractions.Set(keys.Concurrency, c)
	return c
}

// ValidateAndSetMinFreeMem flag verifies the format of the free memory flag.
func ValidateAndSetMinFreeMem(minFreeMem string) {
	if minFreeMem == "" || minFreeMem == "0" {
		return
	}

	var err error
	if minFreeMem, err = sharedvalidation.ValidateMinFreeMem(minFreeMem); err != nil {
		logger.Pl.E("Invalid min free memory setting %q: %v", minFreeMem, err)
	}

	// Strip to base number, set multiply factor.
	multiplyFactor := uint64(1) // Default (bytes).
	switch {
	case strings.HasSuffix(minFreeMem, "G"):
		minFreeMem = strings.TrimSuffix(minFreeMem, "G")
		multiplyFactor = consts.GB
	case strings.HasSuffix(minFreeMem, "M"):
		minFreeMem = strings.TrimSuffix(minFreeMem, "M")
		multiplyFactor = consts.MB
	case strings.HasSuffix(minFreeMem, "K"):
		minFreeMem = strings.TrimSuffix(minFreeMem, "K")
		multiplyFactor = consts.KB
	}

	// Retrieve system RAM.
	currentAvailableMem, err := mem.VirtualMemory()
	if currentAvailableMem == nil {
		currentAvailableMem = &mem.VirtualMemoryStat{}
	}

	if err != nil {
		logger.Pl.E("Could not get system memory, using default max RAM requirements: %v", err)
		currentAvailableMem.Available = consts.GB // Guess 1 GB (conservative).
	}

	minFreeMemInt, err := strconv.Atoi(minFreeMem)
	if err != nil {
		logger.Pl.E("Could not get system memory from invalid argument %q, using default max RAM requirements: %v", minFreeMem, err)
		currentAvailableMem.Available = consts.GB
	}

	ninetyPercent := currentAvailableMem.Available * 9 / 10
	parsedMinFree := min(uint64(minFreeMemInt)*multiplyFactor, ninetyPercent)

	if parsedMinFree > 0 {
		logger.Pl.I("Min RAM to spawn process: %v", parsedMinFree)
	}
	abstractions.Set(keys.MinFreeMem, parsedMinFree)
}

// ValidateAndSetMaxCPU validates and sets the maximum CPU limit.
func ValidateAndSetMaxCPU(maxCPU float64) {
	if maxCPU <= 5.0 {
		maxCPU = 5.0
		logger.Pl.E("Max CPU usage entered too low, setting to default lowest: %.2f%%", maxCPU)
	}

	abstractions.Set(keys.MaxCPU, sharedvalidation.ValidateMaxCPU(maxCPU, false))
}

// ValidateAndSetOutputFiletype verifies the output filetype is valid for FFmpeg.
func ValidateAndSetOutputFiletype(o string) {
	var err error
	if o, err = sharedvalidation.ValidateFFmpegOutputExt(o); err != nil {
		logger.Pl.E("Will not change file containers, invalid output extension: %v", err)
		return
	}

	abstractions.Set(keys.OutputFiletype, o)
	logger.Pl.I("Outputting files as %s", o)
}

// ValidateAndSetMetaOverwritePreserve checks if the entered meta overwrite and preserve flags are valid.
func ValidateAndSetMetaOverwritePreserve(mOverwrite, mPreserve bool) {
	if mOverwrite && mPreserve {
		abstractions.Set(keys.MOverwrite, false)
		abstractions.Set(keys.MPreserve, false)
	}
}

// ValidateAndSetPurgeMetafiles checks and sets the type of metafile purge to perform.
func ValidateAndSetPurgeMetafiles(purgeType string) {
	var e enums.PurgeMetafiles

	// Normalize string.
	purgeType = strings.TrimSpace(purgeType)
	purgeType = strings.ToLower(purgeType)
	purgeType = strings.ReplaceAll(purgeType, ".", "")

	// Compare to list.
	switch purgeType {
	case "all":
		e = enums.PurgeMetaAll
	case "json":
		e = enums.PurgeMetaJSON
	case "nfo":
		e = enums.PurgeMetaNFO
	default:
		e = enums.PurgeMetaNone
	}
	abstractions.Set(keys.MetaPurgeEnum, e)
}

// ValidateAndSetInputFiletypes checks that the inputted filetypes are accepted.
func ValidateAndSetInputFiletypes(argsVInputExts, argsMInputExts []string) {
	inputVExts := make([]string, 0, len(argsVInputExts))
	for _, data := range argsVInputExts {
		// Normalize.
		data = strings.TrimSpace(data)
		data = strings.ToLower(data)
		if !strings.HasPrefix(data, ".") && data != "all" {
			data = "." + data
		}

		switch data {
		case "all":
			inputVExts = append(inputVExts, "all")
		case ".mkv":
			inputVExts = append(inputVExts, data)
		case ".mp4":
			inputVExts = append(inputVExts, data)
		case ".webm":
			inputVExts = append(inputVExts, data)
		default:
			continue
		}
	}
	if len(inputVExts) == 0 {
		inputVExts = append(inputVExts, "all")
	}
	logger.Pl.D(2, "Received video input extension filters: %v", inputVExts)
	abstractions.Set(keys.InputVExts, inputVExts)

	// Metadata extensions.
	inputMExts := make([]string, 0, len(argsMInputExts))
	for _, data := range argsMInputExts {
		switch data {
		case "json", "nfo":
			inputMExts = append(inputMExts, data)
		default:
			continue
		}
	}
	if len(inputMExts) == 0 {
		inputMExts = append(inputMExts, "all")
	}
	logger.Pl.D(2, "Received meta input extension filter: %v", inputMExts)
	abstractions.Set(keys.InputMExts, inputMExts)
}

// ValidateAndSetFileFilters checks and sets the file prefix filters.
func ValidateAndSetFileFilters(viperKey string, argsInputPrefixes []string) {
	if !abstractions.IsSet(viperKey) {
		return
	}
	fileFilters := make([]string, 0, len(argsInputPrefixes))

	for _, arg := range argsInputPrefixes {
		if arg != "" {
			fileFilters = append(fileFilters, arg)
		}
	}
	if len(fileFilters) > 0 {
		abstractions.Set(viperKey, fileFilters)
	}
}

// ValidateAndSetTranscodeQuality validates the transcode quality preset.
func ValidateAndSetTranscodeQuality(q string) error {
	if q == "" {
		return nil
	}

	var err error
	if q, err = sharedvalidation.ValidateTranscodeQuality(q); err != nil {
		return err
	}

	abstractions.Set(keys.TranscodeQuality, q)
	return nil
}

// ValidateAndSetRenameFlag sets the rename style to apply.
func ValidateAndSetRenameFlag(renameFlag string) {
	var renameFlagEnum enums.ReplaceToStyle

	renameFlag = sharedvalidation.GetRenameFlag(renameFlag)

	switch renameFlag {
	case sharedconsts.RenameSpaces:
		renameFlagEnum = enums.RenamingSpaces
		logger.Pl.I("Rename style selected: %v", renameFlag)

	case sharedconsts.RenameUnderscores:
		renameFlagEnum = enums.RenamingUnderscores
		logger.Pl.I("Rename style selected: %v", renameFlag)

	case sharedconsts.RenameFixesOnly:
		renameFlagEnum = enums.RenamingFixesOnly
		logger.Pl.I("Rename style selected: %v", renameFlag)

	case sharedconsts.RenameSkip:
		renameFlagEnum = enums.RenamingSkip
		logger.Pl.I("Rename style selected: %v", renameFlag)

	default:
		logger.Pl.D(1, "'Spaces', 'underscores' or 'fixes-only' not selected for renaming style, skipping these modifications.")
		renameFlagEnum = enums.RenamingSkip
	}
	abstractions.Set(keys.Rename, renameFlagEnum)
}
