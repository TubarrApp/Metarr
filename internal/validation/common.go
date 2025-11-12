// Package validation handles validation of user flag input.
package validation

import (
	"errors"
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/domain/lookupmaps"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
		if _, err := ValidateDirectory(dir, false); err != nil {
			return err
		}
		validatedDirs[dir] = true
	}

	logging.D(1, "Metarr output directories: %+v", outDirMap)
	return nil
}

// ValidateDirectory validates that the directory exists, else creates it if desired.
func ValidateDirectory(dir string, createIfNotFound bool) (os.FileInfo, error) {
	logging.D(3, "Statting directory %q...", dir)
	dir = filepath.Clean(dir)

	// Stat path
	info, err := os.Stat(dir)
	if err == nil { // Err IS nil
		if !info.IsDir() {
			return nil, fmt.Errorf("path %q is a file, not a directory", dir)
		}
		return info, nil
	}

	// Error other than non-existence
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to stat directory %q: %w", dir, err)
	}

	// Does not exist, should not create
	if !createIfNotFound {
		return nil, fmt.Errorf("directory %q does not exist", dir)
	}

	// Generate new directories
	logging.D(3, "Directory %q does not exist, creating it...", dir)
	if err := os.MkdirAll(dir, consts.PermsGenericDir); err != nil {
		return nil, fmt.Errorf("directory %q does not exist and failed to create: %w", dir, err)
	}

	// Stat newly generated directory
	info, err = os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %q", dir)
	}
	return info, nil
}

// ValidateFile validates that the file exists, else creates it if desired.
func ValidateFile(path string, createIfNotFound bool) (os.FileInfo, error) {
	logging.D(3, "Statting file %q...", path)
	path = filepath.Clean(path)

	// Stat path
	info, err := os.Stat(path)
	if err == nil { // Err IS nil
		if info.IsDir() {
			return nil, fmt.Errorf("path %q is a directory, not a file", path)
		}
		return info, nil
	}

	// Error other than non-existence
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to stat file %q: %w", path, err)
	}

	// Does not exist, should not create
	if !createIfNotFound {
		return nil, fmt.Errorf("file %q does not exist", path)
	}

	// Generate new file (must close after os.Create())
	logging.D(3, "File %q does not exist, creating it...", path)
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("file %q does not exist and failed to create: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logging.E("Failed to close file %q: %v", file.Name(), closeErr)
		}
	}()

	// Stat newly generated file
	info, err = os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat created file %q: %w", path, err)
	}
	return info, nil
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

	if !consts.ValidGPUAccelTypes[g] {
		switch g {
		case "amd", "radeon":
			g = consts.AccelTypeAMF
		case "intel":
			g = consts.AccelTypeIntel
		case "nvidia", consts.AccelFlagNvenc:
			g = consts.AccelTypeNvidia
		case "automatic", "automate", "automated":
			g = consts.AccelTypeAuto
		default:
			return "", fmt.Errorf("hardware acceleration flag %q is invalid, aborting", g)
		}
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
// ValidateAndSetBatchPairs retrieves valid files and directories from a batch pair entry.
func ValidateAndSetBatchPairs(batchPairs []string) error {
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
	c = max(c, 1)
	abstractions.Set(keys.Concurrency, c)
	return c
}

// ValidateAndSetMinFreeMem flag verifies the format of the free memory flag.
func ValidateAndSetMinFreeMem(minFreeMem string) {
	if minFreeMem == "" || minFreeMem == "0" {
		return
	}

	minFreeMem = strings.ToUpper(strings.TrimSuffix(minFreeMem, "B"))
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

	parsedMinFree := min(uint64(minFreeMemInt)*multiplyFactor, currentAvailableMem.Available)

	if parsedMinFree > 0 {
		logging.I("Min RAM to spawn process: %v", parsedMinFree)
	}
	abstractions.Set(keys.MinFreeMem, parsedMinFree)
}

// ValidateAndSetMaxCPU validates and sets the maximum CPU limit.
func ValidateAndSetMaxCPU(maxCPU float64) {
	maxCPU = min(maxCPU, 101.0)

	if maxCPU <= 5.0 {
		maxCPU = 5.0
		logging.E("Max CPU usage entered too low, setting to default lowest: %.2f%%", maxCPU)
	}
	abstractions.Set(keys.MaxCPU, maxCPU)
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
		logging.I("Outputting files as %s", o)
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

// ValidateAndSetVideoCodec validates the user input codec selection.
func ValidateAndSetVideoCodec(c string) error {
	c = strings.ToLower(strings.TrimSpace(c))
	c = strings.ReplaceAll(c, ".", "")
	c = strings.ReplaceAll(c, "-", "")
	c = strings.ReplaceAll(c, "_", "")

	// Synonym and alias mapping before acceleration compatability check
	switch c {
	case "aom", "libaom", "libaomav1", "av01", "svtav1", "libsvtav1":
		c = consts.VCodecAV1
	case "x264", "avc", "h264avc", "mpeg4avc", "h264mpeg4", "libx264":
		c = consts.VCodecH264
	case "x265", "h265", "hevc265", "libx265", "hevc":
		c = consts.VCodecHEVC
	case "mpg2", "mpeg2video", "mpeg2v", "mpg", "mpeg", "mpeg2":
		c = consts.VCodecMPEG2
	case "libvpx", "vp08", "vpx", "vpx8":
		c = consts.VCodecVP8
	case "libvpxvp9", "libvpx9", "vpx9", "vp09", "vpxvp9":
		c = consts.VCodecVP9
	}

	// Check codec is in map and valid with set GPU acceleration type
	var gpuType string
	if abstractions.IsSet(keys.UseGPU) {
		gpuType = abstractions.GetString(keys.UseGPU)
	}
	if consts.ValidVideoCodecs[c] {
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
		case consts.AccelTypeIntel:
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
	return fmt.Errorf("video codec %q not supported. Supported codecs: %v", c, consts.ValidVideoCodecs)
}

// ValidateAndSetAudioCodec verifies the audio codec to use for transcode/encode operations.
func ValidateAndSetAudioCodec(a string) error {
	a = strings.ToLower(strings.TrimSpace(a))
	a = strings.ReplaceAll(a, ".", "")
	a = strings.ReplaceAll(a, "-", "")
	a = strings.ReplaceAll(a, "_", "")

	// Search for exact matches
	if consts.ValidAudioCodecs[a] {
		logging.I("Setting audio codec: %q", a)
		abstractions.Set(keys.TranscodeAudioCodec, a)
		return nil
	}

	// Synonym and alias mapping
	switch a {
	case "aac", "aaclc", "m4a", "mp4a", "aaclowcomplexity":
		a = consts.ACodecAAC
	case "alac", "applelossless", "m4aalac":
		a = consts.ACodecALAC
	case "dca", "dts", "dtshd", "dtshdma", "dtsma", "dtsmahd", "dtscodec":
		a = consts.ACodecDTS
	case "ddplus", "dolbydigitalplus", "ac3e", "ec3", "eac3":
		a = consts.ACodecEAC3
	case "flac", "flaccodec", "fla", "losslessflac":
		a = consts.ACodecFLAC
	case "mp2", "mpa", "mpeg2audio", "mpeg2", "m2a", "mp2codec":
		a = consts.ACodecMP2
	case "mp3", "libmp3lame", "mpeg3", "mpeg3audio", "mpg3", "mp3codec":
		a = consts.ACodecMP3
	case "opus", "opuscodec", "oggopus", "webmopus":
		a = consts.ACodecOpus
	case "pcm", "wavpcm", "rawpcm", "pcm16", "pcms16le", "pcms24le", "pcmcodec":
		a = consts.ACodecPCM
	case "truehd", "dolbytruehd", "thd", "truehdcodec":
		a = consts.ACodecTrueHD
	case "vorbis", "oggvorbis", "webmvorbis", "vorbiscodec", "vorb":
		a = consts.ACodecVorbis
	case "wav", "wave", "waveform", "pcmwave", "wavcodec":
		a = consts.ACodecWAV
	default:
		return fmt.Errorf("audio codec %q not supported. Supported codecs: %v", a, consts.ValidAudioCodecs)
	}

	abstractions.Set(keys.TranscodeAudioCodec, a)
	return nil
}

// ValidateAndSetTranscodeQuality validates the transcode quality preset.
func ValidateAndSetTranscodeQuality(q string, accelType string) error {
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

// ValidateAndSetRenameFlag sets the rename style to apply.
func ValidateAndSetRenameFlag(argRenameFlag string) {
	var renameFlag enums.ReplaceToStyle

	argRenameFlag = strings.ToLower(strings.TrimSpace(argRenameFlag))

	switch argRenameFlag {
	case consts.RenameSpaces, "space":
		renameFlag = enums.RenamingSpaces
		logging.I("Rename style selected: %v", argRenameFlag)

	case consts.RenameUnderscores, "underscore":
		renameFlag = enums.RenamingUnderscores
		logging.I("Rename style selected: %v", argRenameFlag)

	case consts.RenameFixesOnly, "fix", "fixes", "fixesonly":
		renameFlag = enums.RenamingFixesOnly
		logging.I("Rename style selected: %v", argRenameFlag)

	default:
		logging.D(1, "'Spaces', 'underscores' or 'fixes-only' not selected for renaming style, skipping these modifications.")
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
