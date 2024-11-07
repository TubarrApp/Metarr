package metadata

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	"Metarr/internal/models"
	logging "Metarr/internal/utils/logging"
	"fmt"
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

// NewffCommandBuilder creates a new FFmpeg command builder
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

	if t.Title == "" && t.FallbackTitle != "" {
		t.Title = t.FallbackTitle
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

	// Array credits
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

	inExt := filepath.Ext(b.inputFile)

	if outExt == "" {
		outExt = inExt
	}

	logging.PrintI("Input extension set '%s' and output extension '%s'. File: %s", inExt, outExt, b.inputFile)

	// Return early with straight copy if no extension change
	if strings.TrimPrefix(inExt, ".") == strings.TrimPrefix(outExt, ".") {
		b.formatFlags = consts.AVCodecCopy[:]
		return
	}

	flags := make([]string, 0)

	// Set flags based on output format requirements
	switch outExt {
	case ".mp4":
		flags = append(flags, "-f", outExt)
		flags = append(flags, consts.VideoToH264Balanced[:]...)
		flags = append(flags, consts.PixelFmtYuv420p[:]...)
		flags = append(flags, consts.AudioToAAC[:]...)
		flags = append(flags, consts.AudioBitrate[:]...)

	case ".mkv":
		flags = append(flags, "-f", outExt)
		// MKV is flexible, copy AV codec for supported formats
		if inExt == ".mp4" || inExt == ".m4v" {
			flags = append(flags, consts.VideoCodecCopy[:]...)
		} else {
			flags = append(flags, consts.VideoToH264Balanced[:]...)
		}
		flags = append(flags, consts.AudioToAAC[:]...)
		flags = append(flags, consts.AudioBitrate[:]...)

	case ".webm":
		flags = append(flags, "-f", outExt)
		flags = append(flags, consts.VideoToH264Balanced[:]...)
		flags = append(flags, consts.PixelFmtYuv420p[:]...)
		flags = append(flags, consts.KeyframeBalanced[:]...)
		flags = append(flags, consts.AudioToAAC[:]...)
		flags = append(flags, consts.AudioBitrate[:]...)

	default:
		// Safe defaults for any other output format
		flags = append(flags, "-f", outExt)
		flags = append(flags, consts.VideoToH264Balanced[:]...)
		flags = append(flags, consts.PixelFmtYuv420p[:]...)
		flags = append(flags, consts.AudioToAAC[:]...)
		flags = append(flags, consts.AudioBitrate[:]...)
	}

	b.formatFlags = flags
}

// buildFinalCommand assembles the final FFmpeg command
func (b *ffCommandBuilder) buildFinalCommand() ([]string, error) {

	args := make([]string, 0, calculateCommandCapacity(b))

	args = append(args, b.gpuAccel...)
	args = append(args, "-y", "-i", b.inputFile)

	// Add all -metadata commands
	for key, value := range b.metadataMap {
		args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, strings.TrimSpace(value)))
	}

	args = append(args, b.formatFlags...)
	args = append(args, b.outputFile)

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

	logging.PrintD(3, "Total command capacity calculated as: %d", totalCapacity)
	return totalCapacity
}
