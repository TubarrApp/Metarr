package ffmpeg

import (
	"fmt"
	"metarr/internal/config"
	consts "metarr/internal/domain/constants"
	enums "metarr/internal/domain/enums"
	keys "metarr/internal/domain/keys"
	"metarr/internal/models"
	logging "metarr/internal/utils/logging"
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
}

// newFfCommandBuilder creates a new FFmpeg command builder
func newFfCommandBuilder(fd *models.FileData, outputFile string) *ffCommandBuilder {
	return &ffCommandBuilder{
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

	if t.LongDescription == "" && t.Long_Description != "" {
		t.LongDescription = t.Long_Description
	}

	fields := map[string]string{
		"title":           t.Title,
		"subtitle":        t.Subtitle,
		"description":     t.Description,
		"longdescription": t.LongDescription,
		"summary":         t.Summary,
		"synopsis":        t.Synopsis,
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
		"actor":     c.Actor,
		"author":    c.Author,
		"artist":    c.Artist,
		"creator":   c.Creator,
		"studio":    c.Studio,
		"publisher": c.Publisher,
		"producer":  c.Producer,
		"performer": c.Performer,
		"composer":  c.Composer,
		"director":  c.Director,
		"writer":    c.Writer,
	}

	for field, value := range fields {
		if value != "" {
			b.metadataMap[field] = value
		}
	}

	// Array credits (length already checked in function)
	b.addArrayMetadata("actor", c.Actors)
	b.addArrayMetadata("composer", c.Composers)
	b.addArrayMetadata("artist", c.Artists)
	b.addArrayMetadata("studio", c.Studios)
	b.addArrayMetadata("performer", c.Performers)
	b.addArrayMetadata("producer", c.Producers)
	b.addArrayMetadata("publisher", c.Publishers)
	b.addArrayMetadata("director", c.Directors)
	b.addArrayMetadata("writer", c.Writers)
}

// addDates adds all date-related metadata
func (b *ffCommandBuilder) addDates(d *models.MetadataDates) {

	fields := map[string]string{
		"creation_time":           d.Creation_Time,
		"date":                    d.Date,
		"originally_available_at": d.Originally_Available_At,
		"release_date":            d.ReleaseDate,
		"upload_date":             d.UploadDate,
		"year":                    d.Year,
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
		"episode_id":    s.Episode_ID,
		"episode_sort":  s.Episode_Sort,
		"season_number": s.Season_Number,
		"season_title":  s.Season_Title,
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
		"hd_video": o.HD_Video,
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

	if exists && existing != "" {
		b.metadataMap[key] = existing + "; " + newValue
	} else {
		b.metadataMap[key] = newValue
	}
}

// setGPUAcceleration sets appropriate GPU acceleration flags
func (b *ffCommandBuilder) setGPUAcceleration() {
	if config.IsSet(keys.GPUEnum) {
		gpuFlag, ok := config.Get(keys.GPUEnum).(enums.SysGPU)
		if ok {
			switch gpuFlag {
			case enums.GPU_NVIDIA:
				b.gpuAccel = consts.NvidiaAccel[:]
			case enums.GPU_AMD:
				b.gpuAccel = consts.AMDAccel[:]
			case enums.GPU_INTEL:
				b.gpuAccel = consts.IntelAccel[:]
			}
		}
	}
}

// setFormatFlags adds commands specific for the extension input and output
func (b *ffCommandBuilder) setFormatFlags(outExt string) {
	inExt := strings.ToLower(filepath.Ext(b.inputFile))
	outExt = strings.ToLower(outExt)

	if outExt == "" || strings.EqualFold(inExt, outExt) {
		b.formatFlags = copyPreset.flags
		return
	}

	logging.I("Input extension: '%s', output extension: '%s', File: %s",
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

// buildFinalCommand assembles the final FFmpeg command
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
		args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, strings.TrimSpace(value)))
	}

	if len(b.formatFlags) > 0 {
		args = append(args, b.formatFlags...)
	}

	if b.outputFile != "" {
		args = append(args, b.outputFile)
	}

	return args, nil
}

// calculateCommandCapacity determines the total length needed for the command
func calculateCommandCapacity(b *ffCommandBuilder) int {
	const (
		inputFlags   = 2 // "-y", "-i"
		inputFile    = 1 // input file
		formatFlag   = 1 // "-codec"
		outputFile   = 1 // output file
		metadataFlag = 1 // "-metadata" for each metadata entry
		keyValuePair = 1 // "key=value" for each metadata entry
	)

	totalCapacity := len(b.gpuAccel) + // GPU acceleration flags if any
		inputFlags + inputFile + // Input related flags and file
		(len(b.metadataMap) * (metadataFlag + keyValuePair)) + // Metadata entries
		len(b.formatFlags) + // Format flags (like -codec copy)
		outputFile

	logging.D(3, "Total command capacity calculated as: %d", totalCapacity)
	return totalCapacity
}