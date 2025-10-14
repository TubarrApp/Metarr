// Package cfg intializes Metarr configurations with Viper, Cobra, etc.
package cfg

import (
	"errors"
	"fmt"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/utils/benchmark"
	"metarr/internal/utils/logging"
	"os"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/mem"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "metarr",
	Short: "metarr is a video and metatagging tool",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Setup benchmarking
		if viper.IsSet(keys.Benchmarking) {
			if benchFiles, err := benchmark.SetupBenchmarking(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			} else {
				viper.Set(keys.BenchFiles, benchFiles)
			}
		}

		// Setup flags from config file
		if viper.IsSet(keys.ConfigPath) {
			configFile := viper.GetString(keys.ConfigPath)

			cInfo, err := os.Stat(configFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed check for config file path: %v\n", err)
				os.Exit(1)
			} else if cInfo.IsDir() {
				fmt.Fprintf(os.Stderr, "config file entered is a directory, should be a file\n")
				os.Exit(1)
			}

			if configFile != "" {
				// load and normalize keys from any Viper-supported config file
				if err := loadConfigFile(configFile); err != nil {
					fmt.Fprintf(os.Stderr, "failed loading config file: %v\n", err)
					os.Exit(1)
				}
			}
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Lookup("help").Changed {
			return nil // Stop further execution if help is invoked
		}
		viper.Set("execute", true)
		return execute()
	},
}

// init sets the initial Viper settings
func init() {
	// Env vars
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("_", "-")) // Convert "video_directory" to "video-directory"

	// Config file
	rootCmd.PersistentFlags().String(keys.ConfigPath, "", "Specify a path to your preset configuration file")
	if err := viper.BindPFlag(keys.ConfigPath, rootCmd.PersistentFlags().Lookup(keys.ConfigPath)); err != nil {
		fmt.Fprintf(os.Stderr, "config file path setting failure: %v\n", err)
		os.Exit(1)
	}

	// Files and directories
	initOrExit(initFilesDirs(),
		"files & dirs initialization failure")

	// System resource related
	initOrExit(initResourceRelated(),
		"config resource element initialization failure")

	// Filtering
	initOrExit(initFiltering(),
		"config filtering initialization failure")

	// All file transformations
	initOrExit(initAllFileTransformers(),
		"config file transformer initialization failure")

	// Filename transformations
	initOrExit(initVideoTransformers(),
		"config video transformer initialization failure")

	// Metadata and metafile manipulation
	initOrExit(initMetaTransformers(),
		"config meta transformer initialization failure")

	// Special functions
	initOrExit(initProgramFunctions(),
		"config program function initialization failure")

	// Text replacement initialization
	initOrExit(initTextReplace(),
		"config text replace initialization failure")
}

// Execute is the primary initializer of Viper
func Execute() error {
	fmt.Println()
	if err := rootCmd.Execute(); err != nil {
		logging.E(0, "Failed to execute cobra")
		return err
	}

	return nil
}

// execute more thoroughly handles settings created in the Viper init
func execute() error {

	// Concurrency
	verifyConcurrencyLimit()

	// Resource usage limits (CPU and memory)
	verifyResourceLimits()

	// File extension settings
	verifyInputFiletypes()

	// File prefix filter settings
	verifyFilePrefixes()

	// Debugging level
	verifyDebugLevel()

	// Filetype to output as
	verifyOutputFiletype()

	// Meta overwrite and preserve flags
	verifyMetaOverwritePreserve()

	// Verify user metafile purge settings
	verifyPurgeMetafiles()

	// Parse and verify the audio codec
	if err := verifyAudioCodec(); err != nil {
		return err
	}

	// Parse GPU settings and set commands
	if err := verifyHWAcceleration(); err != nil {
		return err
	}

	// Ensure no video and metadata location conflicts
	if err := checkFileDirs(); err != nil {
		return err
	}

	if err := initTextReplace(); err != nil {
		return err
	}

	if err := initDateReplaceFormat(); err != nil {
		return err
	}

	return nil
}

type BatchConfig struct {
	ID         int64
	Video      string
	JSON       string
	IsDirs     bool
	SkipVideos bool
}

// checkFileDirConflicts ensures no conflicts in the file and directories entered by the user
func checkFileDirs() error {

	var (
		videoFiles, videoDirs,
		jsonFiles, jsonDirs []string
	)

	videoFileSet := viper.IsSet(keys.VideoFiles)
	videoDirSet := viper.IsSet(keys.VideoDirs)
	jsonFileSet := viper.IsSet(keys.JSONFiles)
	jsonDirSet := viper.IsSet(keys.JSONDirs)

	if videoFileSet {
		videoFiles = viper.GetStringSlice(keys.VideoFiles)
	}

	if videoDirSet {
		videoDirs = viper.GetStringSlice(keys.VideoDirs)
	}

	if jsonFileSet {
		jsonFiles = viper.GetStringSlice(keys.JSONFiles)
	}

	if jsonDirSet {
		jsonDirs = viper.GetStringSlice(keys.JSONDirs)
	}

	if len(videoDirs) > len(jsonDirs) || len(videoFiles) > len(jsonFiles) {
		return errors.New("invalid configuration, please enter a meta directory/file for each video directory/file")
	}

	var tasks []BatchConfig

	vDirCount := 0
	vFileCount := 0

	logging.I("Finding video and JSON directories...")

	// Make directory batches
	if len(videoDirs) > 0 {
		for i := range videoDirs {
			vInfo, err := os.Stat(videoDirs[i])
			if err != nil {
				return err
			}
			if !vInfo.IsDir() {
				return fmt.Errorf("file %q entered instead of directory", vInfo.Name())
			}

			jInfo, err := os.Stat(jsonDirs[i])
			if err != nil {
				return err
			}
			if !jInfo.IsDir() {
				return fmt.Errorf("file %q entered instead of directory", jInfo.Name())
			}

			tasks = append(tasks, BatchConfig{
				Video:  videoDirs[i],
				JSON:   jsonDirs[i],
				IsDirs: true,
			})
			vDirCount++
		}
	}

	logging.I("Got %d directory pairs to process, %d singular JSON directories", vDirCount, len(jsonDirs)-vDirCount)

	// Remnant JSON directories
	if len(jsonDirs) > vDirCount {
		j := jsonDirs[vDirCount:]

		for i := range j {
			jInfo, err := os.Stat(j[i])
			if err != nil {
				return err
			}
			if !jInfo.IsDir() {
				return fmt.Errorf("file %q entered instead of directory", jInfo.Name())
			}

			tasks = append(tasks, BatchConfig{
				JSON:       j[i],
				IsDirs:     true,
				SkipVideos: true,
			})
		}
	}

	logging.I("Finding video and JSON files...")

	// Make file batches
	if len(videoFiles) > 0 {
		for i := range videoFiles {

			logging.D(3, "Checking video file %q ...", videoFiles[i])

			vInfo, err := os.Stat(videoFiles[i])
			if err != nil {
				return err
			}
			if vInfo.IsDir() {
				return fmt.Errorf("directory %q entered instead of file", vInfo.Name())
			}

			logging.D(3, "Checking JSON file %q ...", jsonFiles[i])

			jInfo, err := os.Stat(jsonFiles[i])
			if err != nil {
				return err
			}
			if jInfo.IsDir() {
				return fmt.Errorf("directory %q entered instead of file", jInfo.Name())
			}

			tasks = append(tasks, BatchConfig{
				Video:  videoFiles[i],
				JSON:   jsonFiles[i],
				IsDirs: false,
			})
			vFileCount++
		}

		logging.I("Got %d file pairs to process, %d singular JSON files", vFileCount, len(jsonFiles)-len(videoFiles))

		// Remnant JSON files
		if len(jsonFiles) > vFileCount {
			j := jsonFiles[vFileCount-1:]

			for i := range j {
				jInfo, err := os.Stat(j[i])
				if err != nil {
					return err
				}
				if jInfo.IsDir() {
					return fmt.Errorf("directory %q entered instead of file", jInfo.Name())
				}

				tasks = append(tasks, BatchConfig{
					JSON:       j[i],
					IsDirs:     false,
					SkipVideos: true,
				})
			}
		}
	}

	logging.I("Got %d batch jobs to perform.", len(tasks))
	viper.Set(keys.BatchPairs, tasks)

	return nil
}

// verifyFilePrefixes checks and sets the file prefix filters
func verifyFilePrefixes() {
	if !viper.IsSet(keys.FilePrefixes) {
		return
	}

	argsInputPrefixes := viper.GetStringSlice(keys.FilePrefixes)
	filePrefixes := make([]string, 0, len(argsInputPrefixes))

	for _, arg := range argsInputPrefixes {
		if arg != "" {
			filePrefixes = append(filePrefixes, arg)
		}
	}
	if len(filePrefixes) > 0 {
		viper.Set(keys.FilePrefixes, filePrefixes)
	}
}

// verifyMetaOverwritePreserve checks if the entered meta overwrite and preserve flags are valid
func verifyMetaOverwritePreserve() {
	if GetBool(keys.MOverwrite) && GetBool(keys.MPreserve) {
		logging.E(0, "Cannot enter both meta preserve AND meta overwrite, exiting...")
		os.Exit(1)
	}
}

// verifyDebugLevel checks and sets the debugging level to use
func verifyDebugLevel() {
	debugLevel := viper.GetInt(keys.DebugLevel)
	if debugLevel > 5 {
		debugLevel = 5
	} else if debugLevel < 0 {
		debugLevel = 0
	}
	logging.I("Debugging level: %v", debugLevel)
	viper.Set(keys.DebugLevel, debugLevel)
	logging.Level = int(debugLevel)
}

// verifyInputFiletypes checks that the inputted filetypes are accepted
func verifyInputFiletypes() {
	argsVInputExts := viper.GetStringSlice(keys.InputVideoExts)
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
	viper.Set(keys.InputVExtsEnum, inputVExts)

	argsMInputExts := viper.GetStringSlice(keys.InputMetaExts)
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
	viper.Set(keys.InputMExtsEnum, inputMExts)
}

// verifyHWAcceleration checks and sets HW acceleration to use
func verifyHWAcceleration() error {
	if err := validateGPU(); err != nil {
		return err
	}
	if err := validateTranscodeCodec(); err != nil {
		return err
	}
	if err := validateTranscodeQuality(); err != nil {
		return err
	}
	return nil
}

// verifyConcurrencyLimit checks and ensures correct concurrency limit input
func verifyConcurrencyLimit() {
	maxConcurrentProcesses := viper.GetInt(keys.Concurrency)

	switch {
	case maxConcurrentProcesses < 1:
		maxConcurrentProcesses = 1
		logging.E(2, "Max concurrency set too low, set to minimum value: %d", maxConcurrentProcesses)
	default:
		logging.I("Max concurrency: %d", maxConcurrentProcesses)
	}
	viper.Set(keys.Concurrency, maxConcurrentProcesses)
}

// verifyCPUUsage verifies the value used to limit the CPU needed to spawn a new routine
func verifyResourceLimits() {

	if minFreeMem := viper.GetString(keys.MinFreeMemInput); minFreeMem != "" && minFreeMem != "0" {

		minFreeMem = strings.ToUpper(minFreeMem)
		minFreeMem = strings.TrimSuffix(minFreeMem, "B")

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
		if err != nil {
			logging.E(0, "Could not get system memory, using default max RAM requirements: %v", err)
			currentAvailableMem.Available = consts.GB // Guess 1 gig (conservative)
		}

		minFreeMemInt, err := strconv.Atoi(minFreeMem)
		if err != nil {
			logging.E(0, "Could not get system memory from invalid argument %q, using default max RAM requirements: %v", minFreeMem, err)
			currentAvailableMem.Available = consts.GB
		}

		parsedMinFree := uint64(minFreeMemInt) * multiplyFactor

		if parsedMinFree > currentAvailableMem.Available {
			parsedMinFree = currentAvailableMem.Available
		}

		if parsedMinFree > 0 {
			logging.I("Min RAM to spawn process: %v", parsedMinFree)
		}
		viper.Set(keys.MinFreeMem, parsedMinFree)
	}

	if maxCPUUsage := viper.GetFloat64(keys.MaxCPU); maxCPUUsage != 101.0 {
		switch {
		case maxCPUUsage > 100.0:
			maxCPUUsage = 100.0
			logging.E(2, "Max CPU usage entered too high, setting to default max: %.2f%%", maxCPUUsage)

		case maxCPUUsage < 1.0:
			maxCPUUsage = 10.0
			logging.E(0, "Max CPU usage entered too low, setting to default low: %.2f%%", maxCPUUsage)
		}
		if maxCPUUsage != 100.0 {
			logging.I("Max CPU usage: %.2f%%", maxCPUUsage)
		}
		viper.Set(keys.MaxCPU, maxCPUUsage)
	}
}

// verifyOutputFiletype verifies the output filetype is valid for FFmpeg.
func verifyOutputFiletype() {
	if !viper.IsSet(keys.OutputFiletypeInput) {
		return
	}

	o := GetString(keys.OutputFiletypeInput)
	o = strings.TrimSpace(o)

	if !strings.HasPrefix(o, ".") {
		o = "." + o
		viper.Set(keys.OutputFiletype, o)
	}

	valid := false
	for ext := range consts.AllVidExtensions {
		if o != ext {
			continue
		} else {
			valid = true
			break
		}
	}

	if valid {
		logging.I("Outputting files as %s", o)
	}
}

// verifyPurgeMetafiles checks and sets the type of metafile purge to perform.
func verifyPurgeMetafiles() {
	if !viper.IsSet(keys.MetaPurge) {
		return
	}

	var e enums.PurgeMetafiles
	purgeType := viper.GetString(keys.MetaPurge)

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

	viper.Set(keys.MetaPurgeEnum, e)
}

// verifyAudioCodec verifies the audio codec to use for transcode/encode operations.
func verifyAudioCodec() error {
	if !viper.IsSet(keys.TranscodeAudioCodec) {
		return nil
	}

	a := viper.GetString(keys.TranscodeAudioCodec)
	a = strings.ToLower(a)

	switch a {
	case "aac", "copy":
		viper.Set(keys.TranscodeAudioCodec, a)
	default:
		return fmt.Errorf("audio codec flag %q is not currently implemented in this program, aborting", a)
	}
	return nil
}

// validateGPU validates the user input GPU selection.
func validateGPU() error {
	if !viper.IsSet(keys.UseGPU) {
		return nil
	}

	g := viper.GetString(keys.UseGPU)
	g = strings.ToLower(g)

	switch g {
	case "qsv", "intel":
		viper.Set(keys.UseGPU, "qsv")
	case "amd", "radeon", "vaapi":
		viper.Set(keys.UseGPU, "vaapi")

		if !viper.IsSet(keys.TranscodeDeviceDir) {
			return fmt.Errorf("must specify the GPU directory, e.g. '/dev/dri/renderD128'")
		} else {
			gpuDir := viper.GetString(keys.TranscodeDeviceDir)

			_, err := os.Stat(gpuDir)
			if os.IsNotExist(err) {
				return fmt.Errorf("driver location %q does not appear to exist?", gpuDir)
			}
		}

	case "nvidia", "cuda":
		viper.Set(keys.UseGPU, "cuda")
	case "auto", "automatic", "automate", "automated":
		viper.Set(keys.UseGPU, "auto")
	default:
		return fmt.Errorf("hardware acceleration flag %q is invalid, aborting", g)
	}

	return nil
}

// validateTranscodeCodec validates the user input codec selection.
func validateTranscodeCodec() error {
	if !viper.IsSet(keys.TranscodeCodec) {
		return nil
	}

	c := viper.GetString(keys.TranscodeCodec)
	c = strings.ToLower(c)
	c = strings.ReplaceAll(c, ".", "")

	switch c {
	case "h264", "hevc":
		viper.Set(keys.TranscodeCodec, c)
	case "h265":
		viper.Set(keys.TranscodeCodec, "hevc")
	default:
		return fmt.Errorf("entered codec %q not supported. Tubarr supports h264 and HEVC (h265)", c)
	}
	return nil
}

// validateTranscodeQuality validates the transcode quality preset.
func validateTranscodeQuality() error {
	if !viper.IsSet(keys.TranscodeQuality) {
		return nil
	}

	q := viper.GetString(keys.TranscodeQuality)
	q = strings.ToLower(q)
	q = strings.ReplaceAll(q, " ", "")

	switch q {
	case "p1", "p2", "p3", "p4", "p5", "p6", "p7":
		logging.I("Got transcode quality profile %q", q)
		viper.Set(keys.TranscodeQuality, q)
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

	viper.Set(keys.TranscodeQuality, qualProf)
	return nil
}
