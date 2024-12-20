package ffmpeg

import (
	"metarr/internal/cfg"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/keys"
	"metarr/internal/models"
	"metarr/internal/utils/logging"
	"path/filepath"
	"strings"
)

// ffCommandBuilder handles FFmpeg command construction
type ffCommandBuilder struct {
	inputFile   string
	outputFile  string
	formatFlags []string
	gpuAccel    []string
	metadataMap map[string]string
	builder     *strings.Builder
}

// newFfCommandBuilder creates a new FFmpeg command builder
func newFfCommandBuilder(fd *models.FileData, outputFile string) *ffCommandBuilder {
	return &ffCommandBuilder{
		builder:     &strings.Builder{},
		inputFile:   fd.OriginalVideoPath,
		outputFile:  outputFile,
		metadataMap: make(map[string]string),
	}
}

// buildCommand constructs the complete FFmpeg command
func (b *ffCommandBuilder) buildCommand(fd *models.FileData, outExt string) ([]string, error) {

	b.setGPUAcceleration()
	b.addAllMetadata(fd)
	b.setFormatFlags(outExt)

	// Return the fully appended argument string
	return b.buildFinalCommand()
}

// addAllMetadata combines all metadata into a single map
func (b *ffCommandBuilder) addAllMetadata(fd *models.FileData) {

	b.addTitlesDescs(fd.MTitleDesc)
	b.addCredits(fd.MCredits)
	b.addDates(fd.MDates)
	b.addShowInfo(fd.MShowData)
	b.addOtherMetadata(fd.MOther)
}

// addTitlesDescs adds all title/description-related metadata
func (b *ffCommandBuilder) addTitlesDescs(t *models.MetadataTitlesDescs) {

	// Prefer fulltitle if possible (also exists in the JSON processing func)
	if t.Title == "" && t.Fulltitle != "" {
		t.Title = t.Fulltitle
	}

	if t.LongDescription == "" && t.LongUnderscoreDescription != "" {
		t.LongDescription = t.LongUnderscoreDescription
	}

	fields := map[string]string{
		consts.JTitle:       t.Title,
		consts.JSubtitle:    t.Subtitle,
		consts.JDescription: t.Description,
		consts.JLongDesc:    t.LongDescription,
		consts.JSummary:     t.Summary,
		consts.JSynopsis:    t.Synopsis,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addCredits adds all credit-related metadata
func (b *ffCommandBuilder) addCredits(c *models.MetadataCredits) {

	// Single value credits
	fields := map[string]string{
		consts.JActor:     c.Actor,
		consts.JAuthor:    c.Author,
		consts.JArtist:    c.Artist,
		consts.JCreator:   c.Creator,
		consts.JStudio:    c.Studio,
		consts.JPublisher: c.Publisher,
		consts.JProducer:  c.Producer,
		consts.JPerformer: c.Performer,
		consts.JComposer:  c.Composer,
		consts.JDirector:  c.Director,
		consts.JWriter:    c.Writer,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}

	// Array credits (length already checked in function)
	b.addArrayMetadata(consts.JActor, c.Actors)
	b.addArrayMetadata(consts.JComposer, c.Composers)
	b.addArrayMetadata(consts.JArtist, c.Artists)
	b.addArrayMetadata(consts.JStudio, c.Studios)
	b.addArrayMetadata(consts.JPerformer, c.Performers)
	b.addArrayMetadata(consts.JProducer, c.Producers)
	b.addArrayMetadata(consts.JPublisher, c.Publishers)
	b.addArrayMetadata(consts.JDirector, c.Directors)
	b.addArrayMetadata(consts.JWriter, c.Writers)
}

// addDates adds all date-related metadata
func (b *ffCommandBuilder) addDates(d *models.MetadataDates) {

	fields := map[string]string{
		consts.JCreationTime:        d.CreationTime,
		consts.JDate:                d.Date,
		consts.JOriginallyAvailable: d.OriginallyAvailableAt,
		consts.JReleaseDate:         d.ReleaseDate,
		consts.JUploadDate:          d.UploadDate,
		consts.JYear:                d.Year,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addShowInfo adds all show info related metadata
func (b *ffCommandBuilder) addShowInfo(s *models.MetadataShowData) {

	fields := map[string]string{
		"episode_id":    s.EpisodeID,
		"episode_sort":  s.EpisodeSort,
		"season_number": s.SeasonNumber,
		"season_title":  s.SeasonTitle,
		"show":          s.Show,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addOtherMetadata adds other related metadata
func (b *ffCommandBuilder) addOtherMetadata(o *models.MetadataOtherData) {

	fields := map[string]string{
		"genre":    o.Genre,
		"hd_video": o.HDVideo,
		"language": o.Language,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}
}

// addArrayMetadata combines array values with existing metadata
func (b *ffCommandBuilder) addArrayMetadata(key string, values []string) {
	if len(values) == 0 {
		return
	}

	existing, exists := b.metadataMap[key]
	newValue := strings.Join(values, "; ")

	b.builder.Reset()
	if exists && existing != "" {
		for i, v := range values {
			if i > 0 {
				b.builder.WriteString("; ")
			}
			b.builder.WriteString(v)
		}
	} else {
		b.metadataMap[key] = newValue
	}
}

// setGPUAcceleration sets appropriate GPU acceleration flags
func (b *ffCommandBuilder) setGPUAcceleration() {
	if cfg.IsSet(keys.GPUEnum) {
		gpuFlag, ok := cfg.Get(keys.GPUEnum).(enums.SysGPU)
		if ok {
			switch gpuFlag {
			case enums.GPUNvidia:
				b.gpuAccel = consts.NvidiaAccel[:]
			case enums.GPUAMD:
				b.gpuAccel = consts.AMDAccel[:]
			case enums.GPUIntel:
				b.gpuAccel = consts.IntelAccel[:]
			}
		} else {
			logging.E(0, "Wrong type or null enums.SysGPU. Got: %T", gpuFlag)
		}
	}
}

// setFormatFlags adds commands specific for the extension input and output.
func (b *ffCommandBuilder) setFormatFlags(outExt string) {
	inExt := strings.ToLower(filepath.Ext(b.inputFile))
	outExt = strings.ToLower(outExt)

	if outExt == "" || strings.EqualFold(inExt, outExt) {
		b.formatFlags = copyPreset.flags
		return
	}

	logging.I("Input extension: %q, output extension: %q, File: %s",
		inExt, outExt, b.inputFile)

	// Get format preset from map
	if presets, exists := formatMap[outExt]; exists {
		// Try exact input format match
		if preset, exists := presets[inExt]; exists {
			b.formatFlags = preset.flags
			return
		}
		// Fall back to default preset for this output format
		if preset, exists := presets["*"]; exists {
			b.formatFlags = preset.flags
			return
		}
	}

	// Fall back to copy preset if no mapping found
	b.formatFlags = copyPreset.flags
	logging.D(1, "No format mapping found for %s to %s conversion, using copy preset",
		inExt, outExt)
}

// buildFinalCommand assembles the final FFmpeg command.
func (b *ffCommandBuilder) buildFinalCommand() ([]string, error) {

	args := make([]string, 0, calculateCommandCapacity(b))

	if len(b.gpuAccel) > 0 {
		args = append(args, b.gpuAccel...)
	}

	if b.inputFile != "" {
		args = append(args, "-y", "-i", b.inputFile)
	}

	// Add all -metadata commands
	for key, value := range b.metadataMap {

		// Reset builder
		b.builder.Reset()
		b.builder.WriteString(key)
		b.builder.WriteByte('=')
		b.builder.WriteString(strings.TrimSpace(value))

		// Write argument
		logging.I("Adding metadata argument: '-metadata %s", b.builder.String())
		args = append(args, "-metadata", b.builder.String())
	}

	if len(b.formatFlags) > 0 {
		args = append(args, b.formatFlags...)
	}

	if b.outputFile != "" {
		args = append(args, b.outputFile)
	}

	return args, nil
}

// calculateCommandCapacity determines the total length needed for the command.
func calculateCommandCapacity(b *ffCommandBuilder) int {
	const (
		base = 2 + // "-y", "-i"
			1 + // <input file>
			1 + // "--codec"
			1 // <output file>

		mapArgMultiply = 1 + // "-metadata" for each metadata entry
			1 // "key=value" for each metadata entry
	)

	totalCapacity := base
	totalCapacity += (len(b.metadataMap) * mapArgMultiply)
	totalCapacity += len(b.gpuAccel)
	totalCapacity += len(b.formatFlags)

	logging.D(3, "Total command capacity calculated as: %d", totalCapacity)
	return totalCapacity
}
