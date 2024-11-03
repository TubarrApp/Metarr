package utils

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	logging "Metarr/internal/utils/logging"
	"fmt"
	"net/http"
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

	var (
		result string
		tags   []string
	)

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

		// Check if element matches tags by class, id, etc.
		for _, t := range tags {
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
