// Package parsing handles parsing operations, such as parsing and replacing templating directives.
package parsing

import (
	"errors"
	"fmt"
	"metarr/internal/models"
	"path/filepath"
	"strings"

	"github.com/TubarrApp/gocommon/sharedtemplates"
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

	switch strings.ToLower(tag) {
	case sharedtemplates.MetYear:
		if d.Year != "" {
			return d.Year, nil
		}
		return "", fmt.Errorf("templating: year empty for %q", dp.FD.OriginalVideoPath)

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
		if w.VideoURL != "" {
			return w.VideoURL, nil
		}
		return "", fmt.Errorf("templating: video URL empty for %q", dp.FD.OriginalVideoPath)

	default:
		return "", fmt.Errorf("invalid template tag %q", tag)
	}
}
