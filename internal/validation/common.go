// Package validation handles validation of user flag input.
package validation

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/lookupmaps"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/mem"
	"github.com/spf13/viper"
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
		if _, err := ValidateDirectory(dir, false); err != nil {
			return err
		}
		validatedDirs[dir] = true
	}

	logging.D(1, "Metarr output directories: %q", outDirMap)
	return nil
}

// ValidateDirectory validates that the directory exists, else creates it if desired.
func ValidateDirectory(dir string, createIfNotFound bool) (os.FileInfo, error) {
	logging.D(3, "Statting directory %q...", dir)

	dirInfo, err := os.Stat(dir)
	switch {
	case err == nil: // If err IS nil
		if !dirInfo.IsDir() {
			return dirInfo, fmt.Errorf("path %q is a file, not a directory", dir)
		}
		return dirInfo, nil

	case os.IsNotExist(err):
		if createIfNotFound {
			logging.D(3, "Directory %q does not exist, creating it...", dir)
			if err := os.MkdirAll(dir, consts.PermsGenericDir); err != nil {
				return nil, fmt.Errorf("directory %q does not exist and failed to create: %w", dir, err)
			}
			if dirInfo, err = os.Stat(dir); err != nil { // re-stat to get correct FileInfo
				return dirInfo, fmt.Errorf("failed to stat %q", dir)
			}
			return dirInfo, nil
		}
		return nil, fmt.Errorf("directory %q does not exist", dir)

	default:
		return nil, fmt.Errorf("failed to stat directory %q: %w", dir, err)
	}
}

// ValidateFile validates that the file exists, else creates it if desired.
func ValidateFile(path string, createIfNotFound bool) (os.FileInfo, error) {
	logging.D(3, "Statting file %q...", path)

	info, err := os.Stat(path)
	switch {
	case err == nil: // If err IS nil
		if info.IsDir() {
			return info, fmt.Errorf("path %q is a directory, not a file", path)
		}
		return info, nil

	case os.IsNotExist(err):
		if createIfNotFound {
			logging.D(3, "File %q does not exist, creating it...", path)
			file, err := os.Create(path)
			if err != nil {
				return nil, fmt.Errorf("file %q does not exist and failed to create: %w", path, err)
			}
			if err := file.Close(); err != nil {
				logging.E("Failed to close file %q: %v", file.Name(), err)
			}
			if info, err = os.Stat(path); err != nil {
				return nil, fmt.Errorf("failed to stat created file %q: %w", path, err)
			}
			return info, nil
		}
		return nil, fmt.Errorf("file %q does not exist", path)

	default:
		return nil, fmt.Errorf("failed to stat file %q: %w", path, err)
	}
}

// ValidateBatchPairs retrieves valid files and directories from a batch pair entry.
func ValidateBatchPairs(batchPairs []string) error {
	var vDirs, vFiles, mDirs, mFiles []string
	for _, pair := range batchPairs {
		split := strings.SplitN(pair, ":", 2)
		if len(split) < 2 {
			logging.W("skipping invalid batch pair %q, should be 'video_dir_path:json_dir_path'")
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
	viper.Set(keys.BatchPairs, models.BatchPairs{
		VideoDirs:  vDirs,
		VideoFiles: vFiles,
		MetaDirs:   mDirs,
		MetaFiles:  mFiles,
	})
	return nil
}

// ValidateConcurrencyLimit checks and ensures correct concurrency limit input.
func ValidateConcurrencyLimit(c int) int {
	c = max(c, 1)
	abstractions.Set(keys.Concurrency, c)
	return c
}

// ValidateMinFreeMem flag verifies the format of the free memory flag.
func ValidateMinFreeMem(minFreeMem string) {
	if minFreeMem == "" || minFreeMem == "0" {
		return
	}

	minFreeMem = strings.ToUpper(strings.TrimSuffix(minFreeMem, "B"))
	var multiplyFactor uint64 = 1 // Default (bytes)

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

	currentAvailableMem, err := mem.VirtualMemory()
	if currentAvailableMem == nil {
		currentAvailableMem = &mem.VirtualMemoryStat{}
	}

	if err != nil {
		logging.E("Could not get system memory, using default max RAM requirements: %v", err)
		currentAvailableMem.Available = consts.GB // Guess 1 gig (conservative)
	}

	minFreeMemInt, err := strconv.Atoi(minFreeMem)
	if err != nil {
		logging.E("Could not get system memory from invalid argument %q, using default max RAM requirements: %v", minFreeMem, err)
		currentAvailableMem.Available = consts.GB
	}

	parsedMinFree := uint64(minFreeMemInt) * multiplyFactor

	if parsedMinFree > currentAvailableMem.Available {
		parsedMinFree = currentAvailableMem.Available
	}

	if parsedMinFree > 0 {
		logging.I("Min RAM to spawn process: %v", parsedMinFree)
	}
	abstractions.Set(keys.MinFreeMem, parsedMinFree)
}

// ValidateMaxCPU validates and sets the maximum CPU limit.
func ValidateMaxCPU(maxCPU float64) {
	if maxCPU == 101.0 {
		return
	}

	switch {
	case maxCPU > 100.0:
		maxCPU = 100.0
		logging.E("Max CPU usage entered too high, setting to default max: %.2f%%", maxCPU)

	case maxCPU <= 0.0:
		maxCPU = 0.1
		logging.E("Max CPU usage entered zero, setting to default lowest: %.2f%%", maxCPU)
	}
	if maxCPU != 100.0 {
		logging.I("Max CPU usage: %.2f%%", maxCPU)
	}
	abstractions.Set(keys.MaxCPU, maxCPU)
}

// ValidateOutputFiletype verifies the output filetype is valid for FFmpeg.
func ValidateOutputFiletype(o string) {
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
		logging.I("Outputting files as %s", o)
	}
}

// ValidateMetaOverwritePreserve checks if the entered meta overwrite and preserve flags are valid
func ValidateMetaOverwritePreserve(mOverwrite, mPreserve bool) {
	if mOverwrite && mPreserve {
		abstractions.Set(keys.MOverwrite, false)
		abstractions.Set(keys.MPreserve, false)
	}
}

// ValidatePurgeMetafiles checks and sets the type of metafile purge to perform.
func ValidatePurgeMetafiles(purgeType string) {
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

// WarnMalformedKeys warns a user if a key in their config file is mixed casing.
func WarnMalformedKeys() {
	for _, key := range viper.AllKeys() {
		if strings.Contains(key, "-") && strings.Contains(key, "_") {
			logging.W("Config key %q mixes dashes and underscores - use either kebab-case or snake_case consistently", key)
		}
	}
}

// ValidateInputFiletypes checks that the inputted filetypes are accepted.
func ValidateInputFiletypes(argsVInputExts, argsMInputExts []string) {
	// Video extensions
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
			inputVExts = []string{"all"}
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
		inputVExts = []string{"all"}
	}
	logging.D(2, "Received video input extension filters: %v", inputVExts)
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
	logging.D(2, "Received meta input extension filter: %v", inputMExts)
	abstractions.Set(keys.InputMExts, inputMExts)
}

// ValidateSetFileFilters checks and sets the file prefix filters.
func ValidateSetFileFilters(viperKey string, argsInputPrefixes []string) {
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

// ValidateMaxFilesize checks the max filesize setting.
func ValidateMaxFilesize(m string) (string, error) {
	m = strings.ToUpper(m)
	switch {
	case strings.HasSuffix(m, "B"), strings.HasSuffix(m, "K"), strings.HasSuffix(m, "M"), strings.HasSuffix(m, "G"):
		return strings.TrimSuffix(m, "B"), nil
	default:
		if _, err := strconv.Atoi(m); err != nil {
			return "", err
		}
	}
	return m, nil
}

// ValidateGPU validates the user input GPU selection.
func ValidateGPU(g string) (accelType string, err error) {
	g = strings.ToLower(g)
	logging.I("Checking acceleration type %q", g)
	switch g {
	case consts.AccelTypeQSV, "intel":
		abstractions.Set(keys.UseGPU, consts.AccelTypeQSV)
		if err := checkDriverDirExists(g); err != nil {
			return consts.AccelTypeQSV, err
		}
		accelType = consts.AccelTypeQSV

	case consts.AccelTypeVAAPI:
		abstractions.Set(keys.UseGPU, consts.AccelTypeVAAPI)
		if err := checkDriverDirExists(g); err != nil {
			return consts.AccelTypeVAAPI, err
		}
		accelType = consts.AccelTypeVAAPI

	case consts.AccelTypeAMF, "amd", "radeon":
		abstractions.Set(keys.UseGPU, consts.AccelTypeAMF)
		if err := checkDriverDirExists(g); err != nil {
			return consts.AccelTypeAMF, err
		}
		accelType = consts.AccelTypeAMF

	case consts.AccelTypeNvidia, "nvidia":
		abstractions.Set(keys.UseGPU, consts.AccelTypeNvidia)
		if err := checkDriverDirExists(g); err != nil {
			return consts.AccelTypeNvidia, err
		}
		accelType = consts.AccelTypeNvidia

	case consts.AccelTypeAuto, "automatic", "automate", "automated":
		abstractions.Set(keys.UseGPU, consts.AccelTypeAuto)
		accelType = consts.AccelTypeAuto
	}

	if accelType == "" {
		return "", fmt.Errorf("hardware acceleration flag %q is invalid, aborting", g)
	}
	return accelType, nil
}

// checkDriverDirExists checks the entered driver directory is valid (will NOT show as dir, do not use IsDir check).
func checkDriverDirExists(g string) error {
	if !abstractions.IsSet(keys.TranscodeDeviceDir) {
		return fmt.Errorf("must specify the GPU directory for transcoding of type %q, e.g. '/dev/dri/renderD128'", g)
	}

	gpuDir := abstractions.GetString(keys.TranscodeDeviceDir)

	_, err := os.Stat(gpuDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("driver location %q does not appear to exist?", gpuDir)
	}
	return nil
}

// ValidateVideoCodec validates the user input codec selection.
func ValidateVideoCodec(c string) error {
	c = strings.ToLower(strings.TrimSpace(c))
	c = strings.ReplaceAll(c, ".", "")
	c = strings.ReplaceAll(c, "-", "")

	// Retrieve GPU type
	var gpuType string
	if abstractions.IsSet(keys.UseGPU) {
		gpuType = abstractions.GetString(keys.UseGPU)
	}

	// Direct match first
	if slices.Contains(consts.ValidVideoCodecs, c) {
		switch gpuType {
		case consts.AccelTypeAMF:
			if c == consts.VCodecMPEG2 || c == consts.VCodecVP8 || c == consts.VCodecVP9 {
				logging.W("%q does not support %q codec, will revert to software.", gpuType, c)
				abstractions.Set(keys.UseGPU, "")
			}
		case consts.AccelTypeNvidia:
			if c == consts.VCodecVP8 || c == consts.VCodecVP9 {
				logging.W("%q does not support %q codec, will revert to software.", gpuType, c)
				abstractions.Set(keys.UseGPU, "")
			}
		case consts.AccelTypeQSV:
			if c == consts.VCodecVP8 {
				logging.W("%q does not support %q codec, will revert to software.", gpuType, c)
				abstractions.Set(keys.UseGPU, "")
			}
		case consts.AccelTypeVAAPI:
			if c == consts.VCodecVP8 || c == consts.VCodecVP9 {
				logging.W("%q does not (or does not reliably) support %q codec, will revert to software.", gpuType, c)
				abstractions.Set(keys.UseGPU, "")
			}
		}
		logging.I("Setting video codec type: %q", c)
		abstractions.Set(keys.TranscodeVideoCodec, c)
		return nil
	}

	// Synonym and alias mapping
	switch c {
	case "aom", "libaom", "libaomav1", "av01", "svtav1", "libsvtav1":
		logging.I("Setting video codec type: %q", consts.VCodecAV1)
		abstractions.Set(keys.TranscodeVideoCodec, consts.VCodecAV1)

	case "x264", "avc", "h264avc", "mpeg4avc", "h264mpeg4", "libx264":
		logging.I("Setting video codec type: %q", consts.VCodecH264)
		abstractions.Set(keys.TranscodeVideoCodec, consts.VCodecH264)

	case "x265", "h265", "hevc265", "libx265", "hevc":
		logging.I("Setting video codec type: %q", consts.VCodecHEVC)
		abstractions.Set(keys.TranscodeVideoCodec, consts.VCodecHEVC)

	case "mpg2", "mpeg2video", "mpeg2v", "mpg", "mpeg", "mpeg2":
		logging.I("Setting video codec type: %q", consts.VCodecMPEG2)
		abstractions.Set(keys.TranscodeVideoCodec, consts.VCodecMPEG2)

	case "libvpx", "vp08", "vpx", "vpx8":
		logging.I("Setting video codec type: %q", consts.VCodecVP8)
		abstractions.Set(keys.TranscodeVideoCodec, consts.VCodecVP8)

	case "libvpxvp9", "libvpx9", "vpx9", "vp09", "vpxvp9":
		logging.I("Setting video codec type: %q", consts.VCodecVP9)
		abstractions.Set(keys.TranscodeVideoCodec, consts.VCodecVP9)

	default:
		return fmt.Errorf("video codec %q not supported. Supported codecs: %v", c, consts.ValidVideoCodecs)
	}
	return nil
}

// ValidateAudioCodec verifies the audio codec to use for transcode/encode operations.
func ValidateAudioCodec(c string) error {
	if !abstractions.IsSet(keys.TranscodeAudioCodec) {
		return nil
	}
	c = strings.ToLower(strings.TrimSpace(c))
	c = strings.ReplaceAll(c, ".", "")
	c = strings.ReplaceAll(c, "-", "")

	// Search for exact matches
	if slices.Contains(consts.ValidAudioCodecs, c) {
		logging.I("Setting audio codec: %q", c)
		abstractions.Set(keys.TranscodeAudioCodec, c)
		return nil
	}

	// Synonym and alias mapping
	switch c {
	case "m4a", "mp4a":
		logging.I("Setting audio codec: %q", consts.ACodecAAC)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecAAC)

	case "applelossless", "m4aalac":
		logging.I("Setting audio codec: %q", consts.ACodecALAC)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecALAC)

	case "dca", "dtshd", "dtsma", "dtsmahd":
		logging.I("Setting audio codec: %q", consts.ACodecDTS)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecDTS)

	case "dd+", "dolbydigitalplus", "ac3e", "ec3":
		logging.I("Setting audio codec: %q", consts.ACodecEAC3)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecEAC3)

	case "fla", "losslessflac":
		logging.I("Setting audio codec: %q", consts.ACodecFLAC)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecFLAC)

	case "mpeg2audio", "m2a":
		logging.I("Setting audio codec: %q", consts.ACodecMP2)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecMP2)

	case "mpeg3", "mpeg3audio", "mpg3":
		logging.I("Setting audio codec: %q", consts.ACodecMP3)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecMP3)

	case "oggopus", "webmopus":
		logging.I("Setting audio codec: %q", consts.ACodecOpus)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecOpus)

	case "wavpcm", "rawpcm", "pcm16", "pcms16le":
		logging.I("Setting audio codec: %q", consts.ACodecPCM)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecPCM)

	case "dolbytruehd", "thd":
		logging.I("Setting audio codec: %q", consts.ACodecTrueHD)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecTrueHD)

	case "oggvorbis", "webmvorbis", "vorb":
		logging.I("Setting audio codec: %q", consts.ACodecVorbis)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecVorbis)

	case "wave", "waveform", "pcmwave":
		logging.I("Setting audio codec: %q", consts.ACodecWAV)
		abstractions.Set(keys.TranscodeAudioCodec, consts.ACodecWAV)
	default:
		return fmt.Errorf("audio codec %q not supported. Supported codecs: %v", c, consts.ValidAudioCodecs)
	}
	return nil
}

// ValidateTranscodeQuality validates the transcode quality preset.
func ValidateTranscodeQuality(q string, accelType string) error {
	q = strings.ToLower(q)
	q = strings.ReplaceAll(q, " ", "")
	qNum, err := strconv.ParseInt(q, 10, 64)
	if err != nil {
		return fmt.Errorf("transcode quality input should be numerical")
	}
	qNum = min(max(qNum, 0), 51)

	abstractions.Set(keys.TranscodeQuality, strconv.FormatInt(qNum, 10))
	return nil
}

// ValidateRenameFlag sets the rename style to apply.
func ValidateRenameFlag(argRenameFlag string) {
	var renameFlag enums.ReplaceToStyle

	argRenameFlag = strings.ToLower(strings.TrimSpace(argRenameFlag))

	switch argRenameFlag {
	case "spaces", "space":
		renameFlag = enums.RenamingSpaces
		logging.I("Rename style selected: %v", argRenameFlag)

	case "underscores", "underscore":
		renameFlag = enums.RenamingUnderscores
		logging.I("Rename style selected: %v", argRenameFlag)

	case "fixes", "fix", "fixes-only", "fixesonly":
		renameFlag = enums.RenamingFixesOnly
		logging.I("Rename style selected: %v", argRenameFlag)

	default:
		logging.D(1, "'Spaces', 'underscores' or 'fixes-only' not selected for renaming style, skipping these modifications.")
		renameFlag = enums.RenamingSkip
	}
	abstractions.Set(keys.Rename, renameFlag)
}

// EscapedSplit allows users to escape separator characters without messing up 'strings.Split' logic.
func EscapedSplit(s string, desiredSeparator rune) []string {
	var parts []string
	var buf strings.Builder
	escaped := false

	for _, r := range s {
		switch {
		case escaped:
			// Always take the next character literally
			buf.WriteRune(r)
			escaped = false
		case r == '\\':
			// Escape next character
			escaped = true
		case r == desiredSeparator:
			// Separator
			parts = append(parts, buf.String())
			buf.Reset()
		default:
			buf.WriteRune(r)
		}
	}
	if escaped {
		// Trailing '\' treated as literal backslash
		buf.WriteRune('\\')
	}

	// Add last segment
	parts = append(parts, buf.String())
	return parts
}

// UnescapeSplit reverts string elements back to unescaped versions.
func UnescapeSplit(s string, separatorUsed string) string {
	return strings.ReplaceAll(s, `\`+separatorUsed, separatorUsed)
}

// ValidateExtension checks if the output extension is valid
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
