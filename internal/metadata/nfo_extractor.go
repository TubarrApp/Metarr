package metadata

import (
	"Metarr/internal/logging"
	"Metarr/internal/models"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	titleTags = []string{"<title>", "</title>"}
	descTags  = []string{"<description>", "</description>"}
)

// Processes NFO metadata, attempts to fill blank fields
func ProcessNFOFiles(m *models.FileData, file *os.File) error {

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek NFO file")
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read NFO file content")
	}

	// Fill in titles
	ok, err := fillNFOTitles(m, content)
	if !ok {
		logging.PrintD(2, "Failed to fill titles from NFO")
	} else if err != nil {
		logging.PrintE(0, err.Error())
	}
	return nil
}

// fillNFOTitles populates titles from a read NFO file.
// Please pass the NFO file content retrieve from io.ReadAll
// for this function
func fillNFOTitles(m *models.FileData, content []byte) (bool, error) {

	t := m.MTitleDesc
	var filledData bool = false

	// Title
	title, err := extractNFOTagField(titleTags, content)
	if err != nil {
		logging.PrintE(0, err.Error())

	} else if title != "" {

		t.Title = title
		filledData = true
	}

	// Description
	description, err := extractNFOTagField(descTags, content)
	if err != nil {
		logging.PrintE(0, err.Error())

	} else if description != "" {

		t.LongDescription = description
		t.Description = description
		filledData = true
	}

	return filledData, nil
}

// extractNFOTagField extracts the data between a start and end tag and returns it
func extractNFOTagField(tag []string, content []byte) (string, error) {
	if len(tag) < 2 {
		return "", fmt.Errorf("tag pair unexpected sent in empty or single: '%v'", tag)
	}

	c := string(content)

	start := strings.Index(c, tag[0])
	if start < 0 {
		return "", fmt.Errorf("start tag '%s' not found in content '%s'", tag[0], c)
	}

	end := strings.Index(c[start:], tag[1])
	if end < 0 {
		return "", fmt.Errorf("end tag '%s' not found in content '%s'", tag[1], c)
	}

	returnVal := strings.TrimSpace(c[(start + len(tag[0])) : start+end])

	switch returnVal {
	case "":
		return returnVal, fmt.Errorf("value between tags (%v) is empty", tag)
	default:
		return returnVal, nil
	}
}
