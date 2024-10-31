package utils

import (
	"Metarr/internal/config"
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	keys "Metarr/internal/domain/keys"
	logging "Metarr/internal/utils/logging"
	"bufio"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gocolly/colly"
)

// ScrapeForMetadata searches relevant URLs to try and fill missing metadata
func ScrapeForMetadata(targetURL string, cookies []*http.Cookie, tag enums.WebClassTags) (string, error) {

	c := colly.NewCollector()

	for _, cookie := range cookies {
		c.SetCookies(targetURL, []*http.Cookie{cookie})
	}

	var result string
	var tags []string

	switch tag {
	case enums.WEBCLASS_DATE:
		tags = consts.WebDateTags
	case enums.WEBCLASS_DESCRIPTION:
		tags = consts.WebDescriptionTags
	case enums.WEBCLASS_CREDITS:
		tags = consts.WebCreditsTags
	case enums.WEBCLASS_TITLE:
		tags = consts.WebTitleTags
	default:
		return "", fmt.Errorf("unsupported metadata tag: %v", tag)
	}

	// Navigate to web page and scrape
	c.OnHTML("*", func(e *colly.HTMLElement) {
		if result != "" {
			return // Stop if result is already set
		}

		classAttr := e.Attr("class")
		idAttr := e.Attr("id")
		text := strings.TrimSpace(e.Text)

		// Check if element matches tags based on class, id, or tag type
		for _, t := range tags {
			// Check for <p> tags with specific IDs or for class/id matches
			if (e.Name == "p" && strings.Contains(strings.ToLower(idAttr), t)) ||
				strings.Contains(strings.ToLower(classAttr), t) ||
				strings.Contains(strings.ToLower(idAttr), t) {

				if tag == enums.WEBCLASS_DATE && looksLikeDate(text) {
					result = text
					logging.PrintI("Colly grabbed '%s' from element with class '%s' and id '%s' for URL '%s'",
						result, classAttr, idAttr, targetURL)
					return
				}

				if tag != enums.WEBCLASS_DATE {
					// For non-date tags, directly set result
					result = text
					logging.PrintI("Colly grabbed non-date '%s' from element with class '%s' and id '%s' for URL '%s'",
						result, classAttr, idAttr, targetURL)
					return
				}
			}
		}
	})

	err := c.Visit(targetURL)
	if err != nil {
		return "", fmt.Errorf("error visiting webpage (%s): %v", targetURL, err)
	}

	if result == "" {
		return "", fmt.Errorf("%v not found in the content for URL (%s)", tag, targetURL)
	}

	logging.PrintD(2, "Returning with metadata '%s' for URL '%s'", result, targetURL)
	return result, nil
}

// Check tag if it appears it could contain a date
func looksLikeDate(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) > 30 { // (Dates shouldn't be longer than this)
		return false
	}

	lowered := strings.ToLower(s)

	// Check for month names
	months := []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
	hasMonth := false
	for _, month := range months {
		if strings.Contains(lowered, month) {
			hasMonth = true
			break
		}
	}

	// Check for year
	hasYear := regexp.MustCompile(`\b\d{4}\b`).MatchString(s)

	// Check for day
	hasDay := regexp.MustCompile(`\b\d{1,2}(st|nd|rd|th)?\b`).MatchString(s)

	// Check for common date formats
	datePatterns := []string{
		`\d{1,2}[-/]\d{1,2}[-/]\d{2,4}`, // DD/MM/YYYY or MM/DD/YYYY
		`\d{4}[-/]\d{1,2}[-/]\d{1,2}`,   // YYYY/MM/DD
		`\w+\s+\d{1,2},?\s+\d{4}`,       // Month DD, YYYY
		`\d{1,2}\s+\w+\s+\d{4}`,         // DD Month YYYY
	}

	for _, pattern := range datePatterns {
		if regexp.MustCompile(pattern).MatchString(s) {
			return true
		}
	}

	// If it has at least two of: month, day, year, it's probably a date
	return (hasMonth && hasDay) || (hasMonth && hasYear) || (hasDay && hasYear)
}

// GrabNewEpisodeURLs checks for new episode URLs containing "/episode/" that are not yet in grabbed-urls.txt
func GrabNewEpisodeURLs(targetURL string, cookies []*http.Cookie) ([]string, error) {
	c := colly.NewCollector()

	for _, cookie := range cookies {
		c.SetCookies(targetURL, []*http.Cookie{cookie})
	}

	var episodeURLs []string

	// Scrape all links that contain "/episode/"
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		if strings.Contains(link, "/episode/") {
			episodeURLs = append(episodeURLs, link)
		}
	})

	// Visit the target URL
	err := c.Visit(targetURL)
	if err != nil {
		return nil, fmt.Errorf("error visiting webpage (%s): %v", targetURL, err)
	}

	// Load existing URLs from grabbed-urls.txt
	existingURLs, err := loadURLsFromFile("grabbed-urls.txt")
	if err != nil {
		return nil, fmt.Errorf("error reading grabbed URLs file: %v", err)
	}

	// Filter out URLs that are already in grabbed-urls.txt
	var newURLs []string
	for _, url := range episodeURLs {
		if _, exists := existingURLs[url]; !exists {
			newURLs = append(newURLs, url)
		}
	}

	for _, entry := range newURLs {
		command := consts.GrabLatestCommand(config.GetString(keys.VideoDir), entry)

		logging.PrintI(command.String())

		command.Stdout = os.Stdout
		command.Stderr = os.Stderr

		if err = command.Run(); err != nil {
			logging.PrintE(0, err.Error())
			// EXIT ON COMMAND FAIL
			os.Exit(1)
		}
	}

	// Append new URLs to the file and return them
	if len(newURLs) > 0 {
		if err := appendURLsToFile("grabbed-urls.txt", newURLs); err != nil {
			return nil, fmt.Errorf("error appending new URLs to file: %v", err)
		}
	} else {
		logging.PrintI("No new videos at %s, exiting program...", targetURL)
		// EXIT IF NO NEW VIDEOS
		os.Exit(0)
	}
	return newURLs, nil
}

// loadURLsFromFile reads URLs from a file and returns them as a map for quick lookup
func loadURLsFromFile(filename string) (map[string]struct{}, error) {
	videoDir := config.GetString(keys.VideoDir)
	var filepath string

	switch strings.HasSuffix(videoDir, "/") {
	case false:
		filepath = videoDir + "/" + filename
	default:
		filepath = videoDir + filename
	}

	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	urlMap := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := scanner.Text()
		urlMap[url] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urlMap, nil
}

// appendURLsToFile appends new URLs to the specified file
func appendURLsToFile(filename string, urls []string) error {
	logging.PrintD(2, "Appending URLs to file... %v", urls)
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Track URLs that have already been written
	written := make(map[string]bool)

	// Load existing URLs from the file into the map
	existingFile, err := os.Open(filename)
	if err == nil {
		defer existingFile.Close()
		var line string
		for scanner := bufio.NewScanner(existingFile); scanner.Scan(); {
			line = scanner.Text()
			written[line] = true
		}
	}

	// Append only new URLs to the file
	for _, url := range urls {
		if !written[url] {
			if _, err := file.WriteString(url + "\n"); err != nil {
				return err
			}
			written[url] = true
		}
	}

	return nil
}
