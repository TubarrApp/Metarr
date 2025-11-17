// Package validation handles validation of user flag input.
package validation

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/logger"
	"metarr/internal/domain/lookupmaps"
	"metarr/internal/models"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedvalidation"
	"github.com/shirou/gopsutil/mem"
)

// ValidateMetarrOutputDirs validates the output directories for Metarr.
func ValidateMetarrOutputDirs(urlDirs []string) error {
	if len(urlDirs) == 0 {
		return nil
	}

	// Initialize map and fill from existing
	outDirMap := make(map[string]string)
	validatedDirs := make(map[string]bool, len(urlDirs))

	// Validate directories
	for _, dir := range urlDirs {
		if _, err := sharedvalidation.ValidateDirectory(dir, false); err != nil {
			return err
		}
		validatedDirs[dir] = true
	}

	logger.Pl.D(1, "Metarr output directories: %+v", outDirMap)
	return nil
}

// ValidateGPU validates the user input GPU selection.
func ValidateGPU(g string) (accelType string, err error) {
	if g, err = sharedvalidation.ValidateGPUAccelType(g); err != nil {
		return "", err
	}

	if err := checkDriverDirExists(g); err != nil {
		return g, err
	}
	return g, nil
}

// ValidateExtension checks if the output extension is valid.
func ValidateExtension(ext string) string {
	ext = strings.TrimSpace(ext)

	// Handle empty or invalid cases
	if ext == "" || ext == "." {
		return ""
	}

	// Ensure proper dot prefix
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	// Verify the extension is not just a lone dot
	if len(ext) <= 1 {
		return ""
	}
	return ext
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

	// Deduplicate
	dedupPairs := make([]string, 0, len(pairs))
	for _, p := range pairs {
		if p == "" {
			continue
		}
		if !slices.Contains(dedupPairs, p) {
			dedupPairs = append(dedupPairs, p)
		}
	}

	// Iterate deduped pairs
	for _, p := range dedupPairs {
		split := strings.Split(p, ":")
		input, err := sharedvalidation.ValidateVideoCodec(split[0]) // Safe (split returns non-empty 'p')
		if err != nil {
			return err
		}

		// Singular value, apply to every entry
		if len(split) < 2 {
			for k := range vCodecMap {
				if input == k {
					continue
				}
				vCodecMap[k] = input
			}
			continue
		}

		// Multi value entry, apply specific output to specific input
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

	// Deduplicate
	dedupPairs := make([]string, 0, len(pairs))
	for _, p := range pairs {
		if p == "" {
			continue
		}
		if !slices.Contains(dedupPairs, p) {
			dedupPairs = append(dedupPairs, p)
		}
	}

	// Iterate deduped pairs
	for _, p := range dedupPairs {
		split := strings.Split(p, ":")
		input, err := sharedvalidation.ValidateAudioCodec(split[0]) // Safe (split returns non-empty 'p')
		if err != nil {
			return err
		}

		// Singular value, apply to every entry
		if len(split) < 2 {
			for k := range aCodecMap {
				if input == k {
					continue
				}
				aCodecMap[k] = input
			}
			continue
		}

		// Multi value entry, apply specific output to specific input
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

		// Ensure no colons in names
		if strings.Contains(video, ":") || strings.Contains(meta, ":") {
			return fmt.Errorf("cannot use pair %q\nDO NOT put colons in file or folder names (FFmpeg treats as protocol)", pair)
		}

		// Handle video part
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

		// Check for mismatch
		if vIsDir && !mIsDir || !vIsDir && mIsDir {
			return fmt.Errorf("mismatch in batch entry types: video is dir? %v, meta is dir? %v", vIsDir, mIsDir)
		}

		// Add videos
		if vIsDir {
			vDirs = append(vDirs, video)
		} else {
			vFiles = append(vFiles, video)
		}

		// Add meta
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

	// Strip to base number, set multiply factor
	multiplyFactor := uint64(1) // Default (bytes)
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

	// Retrieve system RAM
	currentAvailableMem, err := mem.VirtualMemory()
	if currentAvailableMem == nil {
		currentAvailableMem = &mem.VirtualMemoryStat{}
	}

	if err != nil {
		logger.Pl.E("Could not get system memory, using default max RAM requirements: %v", err)
		currentAvailableMem.Available = consts.GB // Guess 1 GB (conservative)
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
	o = strings.TrimSpace(o)
	if !strings.HasPrefix(o, ".") {
		o = "." + o
	}

	valid := false
	for ext := range lookupmaps.AllVidExtensions {
		if o != ext {
			continue
		}
		valid = true
		break
	}

	if valid {
		abstractions.Set(keys.OutputFiletype, o)
		logger.Pl.I("Outputting files as %s", o)
	}
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

	// Normalize string
	purgeType = strings.TrimSpace(purgeType)
	purgeType = strings.ToLower(purgeType)
	purgeType = strings.ReplaceAll(purgeType, ".", "")

	// Compare to list
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
		// Normalize
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

	// Metadata extensions
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

// // ValidateAndSetVideoCodec validates the user input codec selection.
// func ValidateAndSetVideoCodec(c string) error {
// 	c = strings.ToLower(strings.TrimSpace(c))
// 	c = strings.ReplaceAll(c, ".", "")
// 	c = strings.ReplaceAll(c, "-", "")
// 	c = strings.ReplaceAll(c, "_", "")

// 	// Synonym and alias mapping before acceleration compatability check
// 	switch c {
// 	case "aom", "libaom", "libaomav1", "av01", "svtav1", "libsvtav1":
// 		c = consts.VCodecAV1
// 	case "x264", "avc", "h264avc", "mpeg4avc", "h264mpeg4", "libx264":
// 		c = consts.VCodecH264
// 	case "x265", "h265", "hevc265", "libx265", "hevc":
// 		c = consts.VCodecHEVC
// 	case "mpg2", "mpeg2video", "mpeg2v", "mpg", "mpeg", "mpeg2":
// 		c = consts.VCodecMPEG2
// 	case "libvpx", "vp08", "vpx", "vpx8":
// 		c = consts.VCodecVP8
// 	case "libvpxvp9", "libvpx9", "vpx9", "vp09", "vpxvp9":
// 		c = consts.VCodecVP9
// 	}

// 	// Check codec is in map and valid with set GPU acceleration type
// 	var gpuType string
// 	if abstractions.IsSet(keys.UseGPU) {
// 		gpuType = abstractions.GetString(keys.UseGPU)
// 	}
// 	if consts.ValidVideoCodecs[c] {
// 		switch gpuType {
// 		case consts.AccelTypeAMF:
// 			if c == consts.VCodecMPEG2 || c == consts.VCodecVP8 || c == consts.VCodecVP9 {
// 				logger.Pl.W("%q does not support %q codec, will revert to software.", gpuType, c)
// 				abstractions.Set(keys.UseGPU, "")
// 			}
// 		case consts.AccelTypeNvidia:
// 			if c == consts.VCodecVP8 || c == consts.VCodecVP9 {
// 				logger.Pl.W("%q does not support %q codec, will revert to software.", gpuType, c)
// 				abstractions.Set(keys.UseGPU, "")
// 			}
// 		case consts.AccelTypeIntel:
// 			if c == consts.VCodecVP8 {
// 				logger.Pl.W("%q does not support %q codec, will revert to software.", gpuType, c)
// 				abstractions.Set(keys.UseGPU, "")
// 			}
// 		case consts.AccelTypeVAAPI:
// 			if c == consts.VCodecVP8 || c == consts.VCodecVP9 {
// 				logger.Pl.W("%q does not (or does not reliably) support %q codec, will revert to software.", gpuType, c)
// 				abstractions.Set(keys.UseGPU, "")
// 			}
// 		}
// 		logger.Pl.I("Setting video codec type: %q", c)
// 		abstractions.Set(keys.TranscodeVideoCodec, c)
// 		return nil
// 	}
// 	return fmt.Errorf("video codec %q not supported. Supported codecs: %v", c, consts.ValidVideoCodecs)
// }

// // ValidateAndSetAudioCodec verifies the audio codec to use for transcode/encode operations.
// func ValidateAndSetAudioCodec(a string) error {
// 	a = strings.ToLower(strings.TrimSpace(a))
// 	a = strings.ReplaceAll(a, ".", "")
// 	a = strings.ReplaceAll(a, "-", "")
// 	a = strings.ReplaceAll(a, "_", "")

// 	// Search for exact matches
// 	if consts.ValidAudioCodecs[a] {
// 		logger.Pl.I("Setting audio codec: %q", a)
// 		abstractions.Set(keys.TranscodeAudioCodec, a)
// 		return nil
// 	}

// 	// Synonym and alias mapping
// 	switch a {
// 	case "aac", "aaclc", "m4a", "mp4a", "aaclowcomplexity":
// 		a = consts.ACodecAAC
// 	case "alac", "applelossless", "m4aalac":
// 		a = consts.ACodecALAC
// 	case "dca", "dts", "dtshd", "dtshdma", "dtsma", "dtsmahd", "dtscodec":
// 		a = consts.ACodecDTS
// 	case "ddplus", "dolbydigitalplus", "ac3e", "ec3", "eac3":
// 		a = consts.ACodecEAC3
// 	case "flac", "flaccodec", "fla", "losslessflac":
// 		a = consts.ACodecFLAC
// 	case "mp2", "mpa", "mpeg2audio", "mpeg2", "m2a", "mp2codec":
// 		a = consts.ACodecMP2
// 	case "mp3", "libmp3lame", "mpeg3", "mpeg3audio", "mpg3", "mp3codec":
// 		a = consts.ACodecMP3
// 	case "opus", "opuscodec", "oggopus", "webmopus":
// 		a = consts.ACodecOpus
// 	case "pcm", "wavpcm", "rawpcm", "pcm16", "pcms16le", "pcms24le", "pcmcodec":
// 		a = consts.ACodecPCM
// 	case "truehd", "dolbytruehd", "thd", "truehdcodec":
// 		a = consts.ACodecTrueHD
// 	case "vorbis", "oggvorbis", "webmvorbis", "vorbiscodec", "vorb":
// 		a = consts.ACodecVorbis
// 	case "wav", "wave", "waveform", "pcmwave", "wavcodec":
// 		a = consts.ACodecWAV
// 	default:
// 		return fmt.Errorf("audio codec %q not supported. Supported codecs: %v", a, consts.ValidAudioCodecs)
// 	}

// 	abstractions.Set(keys.TranscodeAudioCodec, a)
// 	return nil
// }

// ValidateAndSetTranscodeQuality validates the transcode quality preset.
func ValidateAndSetTranscodeQuality(q string, accelType string) error {
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
func ValidateAndSetRenameFlag(argRenameFlag string) {
	var renameFlag enums.ReplaceToStyle

	argRenameFlag = strings.ToLower(strings.TrimSpace(argRenameFlag))

	switch argRenameFlag {
	case consts.RenameSpaces, "space":
		renameFlag = enums.RenamingSpaces
		logger.Pl.I("Rename style selected: %v", argRenameFlag)

	case consts.RenameUnderscores, "underscore":
		renameFlag = enums.RenamingUnderscores
		logger.Pl.I("Rename style selected: %v", argRenameFlag)

	case consts.RenameFixesOnly, "fix", "fixes", "fixesonly":
		renameFlag = enums.RenamingFixesOnly
		logger.Pl.I("Rename style selected: %v", argRenameFlag)

	default:
		logger.Pl.D(1, "'Spaces', 'underscores' or 'fixes-only' not selected for renaming style, skipping these modifications.")
		renameFlag = enums.RenamingSkip
	}
	abstractions.Set(keys.Rename, renameFlag)
}

// ---- Private ----------------------------------------------------------------------------------------------------
// checkDriverDirExists checks the entered driver directory is valid (will NOT show as dir, do not use IsDir check).
func checkDriverDirExists(g string) error {
	if g == consts.AccelTypeAuto {
		return nil // No directory required
	}

	if !abstractions.IsSet(keys.TranscodeDeviceDir) {
		return fmt.Errorf("must specify the GPU directory (e.g. '/dev/dri/renderD128') for transcoding of type %q", g)
	}

	gpuDir := abstractions.GetString(keys.TranscodeDeviceDir)
	if _, err := os.Stat(gpuDir); os.IsNotExist(err) {
		return fmt.Errorf("driver location %q does not appear to exist?", gpuDir)
	}
	return nil
}
