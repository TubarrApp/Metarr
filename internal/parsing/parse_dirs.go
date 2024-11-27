// Package parsing handles parsing operations, such as parsing and replacing templating directives.
package parsing

import (
	"errors"
	"fmt"
	"metarr/internal/domain/templates"
	"metarr/internal/models"
	"path/filepath"
	"strings"
)

const (
	open          = "{{"
	close         = "}}"
	avgReplaceLen = 32
	templateLen   = len(open) + len(close) + 4
)

type Directory struct {
	FD *models.FileData
}

func NewDirectoryParser(fd *models.FileData) *Directory {
	return &Directory{
		FD: fd,
	}
}

// ParseDirectory returns the absolute directory path with template replacements.
func (dp *Directory) ParseDirectory(dir string) (parsedDir string, err error) {
	if dir == "" {
		return "", errors.New("directory sent in empty")
	}

	parsed := dir
	if strings.Contains(dir, open) {
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
func (dp *Directory) parseTemplate(dir string) (string, error) {
	opens := strings.Count(dir, open)
	closes := strings.Count(dir, close)
	if opens != closes {
		return "", fmt.Errorf("mismatched template delimiters: %d opens, %d closes", opens, closes)
	}

	var b strings.Builder
	b.Grow(len(dir) - (opens * templateLen) + (opens * avgReplaceLen)) // Approximate size
	remaining := dir

	for i := 0; i < opens; i++ {
		startIdx := strings.Index(remaining, open)
		if startIdx == -1 {
			return "", fmt.Errorf("%q is missing opening delimiter", remaining)
		}

		endIdx := strings.Index(remaining, close)
		if endIdx == -1 {
			return "", fmt.Errorf("%q is missing closing delimiter", remaining)
		}

		// String up to template open
		b.WriteString(remaining[:startIdx])

		// Replacement string
		tag := remaining[startIdx+len(open) : endIdx]
		replacement, err := dp.replace(strings.TrimSpace(tag))
		if err != nil {
			return "", err
		}
		b.WriteString(replacement)

		// String after template close
		remaining = remaining[endIdx+len(close):]
	}

	// Write any remaining text after last template
	b.WriteString(remaining)

	return b.String(), nil
}

// replace makes template replacements in the directory string.
func (dp *Directory) replace(tag string) (string, error) {

	if dp.FD == nil {
		return "", errors.New("null FileData model")
	}

	d := dp.FD.MDates
	c := dp.FD.MCredits
	w := dp.FD.MWebData

	switch strings.ToLower(tag) {
	case templates.Year:
		if d.Year != "" {
			return d.Year, nil
		}
		return "", fmt.Errorf("templating: year empty for %q", dp.FD.OriginalVideoBaseName)

	case templates.Author:
		if c.Author != "" {
			return c.Author, nil
		}
		return "", fmt.Errorf("templating: author empty for %q", dp.FD.OriginalVideoBaseName)

	case templates.Director:
		if c.Director != "" {
			return c.Director, nil
		}
		return "", fmt.Errorf("templating: director empty for %q", dp.FD.OriginalVideoBaseName)

	case templates.Domain:
		if w.Domain != "" {
			return w.Domain, nil
		}
		return "", fmt.Errorf("templating: domain empty for %q", dp.FD.OriginalVideoBaseName)

	default:
		return "", fmt.Errorf("invalid template tag %q", tag)
	}
}
