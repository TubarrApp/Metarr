// Package parsing handles parsing operations, such as parsing and replacing templating directives.
package parsing

import (
	"cmp"
	"errors"
	"fmt"
	"metarr/internal/models"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TubarrApp/gocommon/sharedconsts"
	"github.com/TubarrApp/gocommon/sharedparsing"
	"github.com/TubarrApp/gocommon/sharedtemplates"
	"github.com/araddon/dateparse"
)

// DirectoryParser is used to access and use directory parsing elements.
type DirectoryParser struct {
	FD *models.FileData
}

// NewDirectoryParser generates a directory parser containing the FileData model.
func NewDirectoryParser(fd *models.FileData) *DirectoryParser {
	return &DirectoryParser{
		FD: fd,
	}
}

// ParseDirectory returns the absolute directory path with template replacements.
func (dp *DirectoryParser) ParseDirectory(dir string) (parsedDir string, err error) {
	if dir == "" {
		return "", errors.New("directory sent in empty")
	}

	const openTag = "{{"
	parsed := dir
	if strings.Contains(dir, openTag) {
		var err error

		parsed, err = dp.parseTemplate(dir)
		if err != nil {
			return "", fmt.Errorf("template parsing error: %w", err)
		}
	}

	abs, err := filepath.Abs(parsed)
	if err != nil {
		return dir, err
	}

	return filepath.Clean(abs), nil
}

// parseTemplate parses template options inside the directory string.
//
// Returns error if the desired data isn't present, to prevent unexpected results for the user.
func (dp *DirectoryParser) parseTemplate(dir string) (string, error) {
	const (
		openTag     = "{{"
		closeTag    = "}}"
		templateLen = len(openTag) + len(closeTag) + 4
	)
	opens := strings.Count(dir, openTag)
	closes := strings.Count(dir, closeTag)
	if opens != closes {
		return "", fmt.Errorf("mismatched template delimiters: %d opens, %d closes", opens, closes)
	}

	var b strings.Builder
	b.Grow(len(dir) - (opens * templateLen) + (opens * 32)) // Approximate size.
	remaining := dir

	for range opens {
		startIdx := strings.Index(remaining, openTag)
		if startIdx == -1 {
			return "", fmt.Errorf("%q is missing opening delimiter", remaining)
		}

		endIdx := strings.Index(remaining, closeTag)
		if endIdx == -1 {
			return "", fmt.Errorf("%q is missing closing delimiter", remaining)
		}

		// String up to template open.
		b.WriteString(remaining[:startIdx])

		// Replacement string.
		tag := remaining[startIdx+len(openTag) : endIdx]
		replacement, err := dp.replace(strings.TrimSpace(tag))
		if err != nil {
			return "", err
		}
		if replacement = sharedparsing.SanitizePathComponent(replacement, sharedconsts.MaxNameBytes); replacement == "" {
			return "", fmt.Errorf("templating: %q is empty once made safe for a path, for %q", tag, dp.FD.OriginalVideoPath)
		}
		b.WriteString(replacement)

		// String after template close.
		remaining = remaining[endIdx+len(closeTag):]
	}

	// Write any remaining text after last template.
	b.WriteString(remaining)

	return b.String(), nil
}

// replace makes template replacements in the directory string.
func (dp *DirectoryParser) replace(tag string) (string, error) {

	if dp.FD == nil {
		return "", errors.New("null FileData model")
	}

	t := dp.FD.MTitleDesc
	d := dp.FD.MDates
	c := dp.FD.MCredits
	w := dp.FD.MWebData

	name, directive := sharedtemplates.ParseTag(tag)
	if directive != "" && !sharedtemplates.ValidTag(tag, sharedtemplates.MetarrTemplateTags) {
		return "", fmt.Errorf("invalid template tag %q, supported tags: %v", tag, sharedtemplates.DescribeTags(sharedtemplates.MetarrTemplateTags))
	}

	switch name {
	case sharedtemplates.MetYear:
		if len(d.Year) != 4 {
			return "", fmt.Errorf("templating: year empty or invalid for %q", dp.FD.OriginalVideoPath)
		}
		if directive == sharedtemplates.YearShort {
			return d.Year[2:], nil
		}
		return d.Year, nil

	case sharedtemplates.MetMonth:
		date, err := dp.date()
		if err != nil {
			return "", err
		}
		switch directive {
		case sharedtemplates.MonthAbbreviated:
			return date.Format("Jan"), nil
		case sharedtemplates.MonthWord:
			return date.Format("January"), nil
		default:
			return date.Format("01"), nil
		}

	case sharedtemplates.MetDay:
		date, err := dp.date()
		if err != nil {
			return "", err
		}
		return date.Format("02"), nil

	case sharedtemplates.MetAuthor:
		if c.Author != "" {
			return c.Author, nil
		}
		return "", fmt.Errorf("templating: author empty for %q", dp.FD.OriginalVideoPath)

	case sharedtemplates.MetDirector:
		if c.Director != "" {
			return c.Director, nil
		}
		return "", fmt.Errorf("templating: director empty for %q", dp.FD.OriginalVideoPath)

	case sharedtemplates.MetDomain:
		if w.Domain != "" {
			return w.Domain, nil
		}
		return "", fmt.Errorf("templating: domain empty for %q", dp.FD.OriginalVideoPath)

	case sharedtemplates.MetVideoTitle:
		if t.Title != "" {
			return t.Title, nil
		}
		return "", fmt.Errorf("templating: title empty for %q", dp.FD.OriginalVideoPath)

	case sharedtemplates.MetVideoURL:
		if u := cmp.Or(w.WebpageURL, w.VideoURL); u != "" { // The page, not yt-dlp's "url" media stream, when known.
			return sharedparsing.StripURLScheme(u), nil
		}
		return "", fmt.Errorf("templating: video URL empty for %q", dp.FD.OriginalVideoPath)

	default:
		return "", fmt.Errorf("invalid template tag %q", tag)
	}
}

// date returns the video's date for month and day tags, from the first date field agreeing with the year tag.
func (dp *DirectoryParser) date() (time.Time, error) {
	d := dp.FD.MDates
	for _, s := range []string{d.Date, d.UploadDate, d.ReleaseDate, d.OriginallyAvailableAt, d.FormattedDate, d.CreationTime} {
		if len(s) < 6 {
			continue
		}
		parsed, err := dateparse.ParseAny(s)
		if err != nil || (d.Year != "" && strconv.Itoa(parsed.Year()) != d.Year) {
			continue
		}
		return parsed, nil
	}
	if d.Year != "" {
		return time.Time{}, fmt.Errorf("templating: no full date from %s for %q", d.Year, dp.FD.OriginalVideoPath)
	}
	return time.Time{}, fmt.Errorf("templating: no full date for %q", dp.FD.OriginalVideoPath)
}
