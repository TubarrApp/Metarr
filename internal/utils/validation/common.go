// Package validation handles validation of user flag input.
package validation

import (
	"fmt"
	"metarr/internal/abstractions"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/utils/logging"
	"os"
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
	// Check directory existence
	dirInfo, err := os.Stat(dir)
	switch {
	case err == nil: // If err IS nil

		if !dirInfo.IsDir() {
			return dirInfo, fmt.Errorf("path %q is a file, not a directory", dir)
		}
		return dirInfo, nil

	case os.IsNotExist(err):
		// path does not exist
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
		// other error
		return nil, fmt.Errorf("failed to stat directory %q: %w", dir, err)
	}
}

// ValidateFile validates that the file exists, else creates it if desired.
func ValidateFile(f string, createIfNotFound bool) (os.FileInfo, error) {
	logging.D(3, "Statting file %q...", f)
	fileInfo, err := os.Stat(f)
	if err != nil {
		if os.IsNotExist(err) {
			switch {
			case createIfNotFound:
				logging.D(3, "File %q does not exist, creating it...", f)
				if _, err := os.Create(f); err != nil {
					return fileInfo, fmt.Errorf("file %q does not exist and Metarr failed to create it: %w", f, err)
				}
			default:
				return fileInfo, fmt.Errorf("file %q does not exist: %w", f, err)
			}
		} else {
			return fileInfo, fmt.Errorf("failed to stat file %q: %w", f, err)
		}
	}

	if fileInfo != nil {
		if fileInfo.IsDir() {
			return fileInfo, fmt.Errorf("file entered %q is a directory", f)
		}
	}

	return fileInfo, nil
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
	for ext := range consts.AllVidExtensions {
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

	purgeType = strings.TrimSpace(purgeType)
	purgeType = strings.ToLower(purgeType)
	purgeType = strings.ReplaceAll(purgeType, ".", "")

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

// ValidateAudioCodec verifies the audio codec to use for transcode/encode operations.
func ValidateAudioCodec(codec string) error {
	if !abstractions.IsSet(keys.TranscodeAudioCodec) {
		return nil
	}
	codec = strings.ToLower(codec)

	switch codec {
	case "aac", "copy":
		abstractions.Set(keys.TranscodeAudioCodec, codec)
	default:
		return fmt.Errorf("audio codec flag %q is not currently implemented in this program, aborting", codec)
	}
	return nil
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
	inputVExts := make([]enums.ConvertFromFiletype, 0, len(argsVInputExts))
	for _, data := range argsVInputExts {
		switch data {
		case "mkv":
			inputVExts = append(inputVExts, enums.VidExtsMKV)
		case "mp4":
			inputVExts = append(inputVExts, enums.VidExtsMP4)
		case "webm":
			inputVExts = append(inputVExts, enums.VidExtsWebM)
		default:
			inputVExts = append(inputVExts, enums.VidExtsAll)
		}
	}
	if len(inputVExts) == 0 {
		inputVExts = append(inputVExts, enums.VidExtsAll)
	}
	logging.D(2, "Received video input extension filter: %v", inputVExts)
	abstractions.Set(keys.InputVExtsEnum, inputVExts)

	// Metadata extensions
	inputMExts := make([]enums.MetaFiletypeFilter, 0, len(argsMInputExts))
	for _, data := range argsMInputExts {
		switch data {
		case "json":
			inputMExts = append(inputMExts, enums.MetaExtsJSON)
		case "nfo":
			inputMExts = append(inputMExts, enums.MetaExtsNFO)
		default:
			inputMExts = append(inputMExts, enums.MetaExtsAll)
		}
	}
	if len(inputMExts) == 0 {
		inputMExts = append(inputMExts, enums.MetaExtsAll)
	}
	logging.D(2, "Received meta input extension filter: %v", inputMExts)
	abstractions.Set(keys.InputMExtsEnum, inputMExts)
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
func ValidateGPU(g string) error {
	g = strings.ToLower(g)

	switch g {
	case "qsv", "intel":
		abstractions.Set(keys.UseGPU, "qsv")
		if err := checkDriverDirExists(g); err != nil {
			return err
		}

	case "amd", "radeon", "vaapi":
		abstractions.Set(keys.UseGPU, "vaapi")
		if err := checkDriverDirExists(g); err != nil {
			return err
		}

	case "nvidia", "cuda":
		abstractions.Set(keys.UseGPU, "cuda")
		if err := checkDriverDirExists(g); err != nil {
			return err
		}

	case "auto", "automatic", "automate", "automated":
		abstractions.Set(keys.UseGPU, "auto")
	default:
		return fmt.Errorf("hardware acceleration flag %q is invalid, aborting", g)
	}

	return nil
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

// ValidateTranscodeCodec validates the user input codec selection.
func ValidateTranscodeCodec(c string) error {
	c = strings.ToLower(c)
	c = strings.ReplaceAll(c, ".", "")

	switch c {
	case "h264", "hevc":
		abstractions.Set(keys.TranscodeCodec, c)
	case "x265", "h265":
		abstractions.Set(keys.TranscodeCodec, "hevc")
	case "x264", "avc":
		abstractions.Set(keys.TranscodeCodec, "h264")
	default:
		return fmt.Errorf("entered codec %q not supported. Metarr supports h264 and HEVC (h265)", c)
	}
	return nil
}

// ValidateTranscodeQuality validates the transcode quality preset.
func ValidateTranscodeQuality(q string) error {
	q = strings.ToLower(q)
	q = strings.ReplaceAll(q, " ", "")

	switch q {
	case "p1", "p2", "p3", "p4", "p5", "p6", "p7":
		logging.I("Got transcode quality profile %q", q)
		abstractions.Set(keys.TranscodeQuality, q)
		return nil
	}

	qNum, err := strconv.Atoi(q)
	if err != nil {
		return fmt.Errorf("input should be p1 to p7, validation of transcoder quality failed")
	}

	var qualProf string
	switch {
	case qNum < 0:
		qualProf = "p1"
	case qNum > 7:
		qualProf = "p7"
	default:
		qualProf = "p" + strconv.Itoa(qNum)
	}
	logging.I("Got transcode quality profile %q", qualProf)

	abstractions.Set(keys.TranscodeQuality, qualProf)
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
