package utils

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	logging "Metarr/internal/utils/logging"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

// ScrapeForMetadata searches relevant URLs to try and fill missing metadata
func ScrapeForMetadata(targetURL string, cookies []*http.Cookie, tag enums.WebClassTags) (string, error) {

	var (
		tags        []string
		selector    map[string]string
		result      string
		scrapeError error
		resultChan  chan string
	)

	c := colly.NewCollector(
		colly.MaxDepth(1),
		colly.Async(true),
	)

	c.SetRequestTimeout(15 * time.Second)

	if len(cookies) > 0 {
		c.SetCookies(targetURL, cookies)
	}

	switch tag {
	case enums.WEBCLASS_DATE:
		tags = consts.WebDateTags[:]

	case enums.WEBCLASS_DESCRIPTION:
		tags = consts.WebDescriptionTags[:]

	case enums.WEBCLASS_CREDITS:
		tags = consts.WebCreditsTags[:]
		selector = consts.WebCreditsSelectors

	case enums.WEBCLASS_TITLE:
		tags = consts.WebTitleTags[:]
	default:
		return "", fmt.Errorf("unsupported metadata tag: %v", tag)
	}

	// Set up error handler
	c.OnError(func(r *colly.Response, err error) {
		scrapeError = fmt.Errorf("failed to scrape %s: %v", r.Request.URL, err)
	})

	// Primary element scraping based on tags
	c.OnHTML("*", func(e *colly.HTMLElement) {
		if result != "" {
			return
		}

		classAttr := strings.ToLower(e.Attr("class"))
		idAttr := strings.ToLower(e.Attr("id"))
		text := strings.TrimSpace(e.Text)

		if classAttr != "" {
			logging.PrintD(2, "Checking element with class: '%s'", classAttr)
		}

		for _, t := range tags {
			if (e.Name == "p" && strings.Contains(idAttr, t)) ||
				strings.Contains(classAttr, t) ||
				strings.Contains(idAttr, t) {

				if tag == enums.WEBCLASS_DATE && !looksLikeDate(text) {
					continue
				}

				result = text
				logging.PrintI("Found '%s' in element with class '%s' and id '%s' for URL '%s'",
					result, classAttr, idAttr, targetURL)
				return
			}
		}
	})

	// Nested search if rudimentary tag search fails
	if selector != nil {
		resultChan = tryNestedScrape(c, selector)
	}

	// Visit with retries
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := c.Visit(targetURL); err != nil {
			return "", err
		}
		c.Wait() // Wait for async requests to complete

		// Check for nested scrape result
		if resultChan != nil {
			select {
			case result = <-resultChan:
				if result != "" {
					return result, nil
				}
			default:
			}
		}

		// Got result from regular scraping
		if result != "" {
			logging.PrintD(2, "Successfully scraped metadata '%s' for URL '%s'", result, targetURL)
			return result, nil
		}

		if attempt < maxRetries-1 {
			time.Sleep(time.Second * time.Duration(attempt+1))
			logging.PrintD(1, "Retry attempt %d for URL '%s'", attempt+1, targetURL)
		}
	}

	if scrapeError != nil {
		return "", scrapeError
	}

	return "", fmt.Errorf("%v not found in the content for URL (%s)", tag, targetURL)
}

// tryNestedScrape attempts to find content in nested elements
func tryNestedScrape(c *colly.Collector, selector map[string]string) chan string {
	resultChan := make(chan string, 1)

	if selector != nil {
		fmt.Println()
		logging.PrintD(2, "Have selectors: %v", selector)

		for outer, inner := range selector {
			c.OnHTML(outer, func(e *colly.HTMLElement) {
				if strings.Contains(outer, "script") {
					// Handle JSON content
					var schema struct {
						Author struct {
							Name string `json:"name"`
						} `json:"author"`
					}
					if err := json.Unmarshal([]byte(e.Text), &schema); err == nil {
						resultChan <- schema.Author.Name
						logging.PrintI("Found JSON content: '%s'", schema.Author.Name)
					}
				} else {
					// Handle DOM elements
					e.ForEach("."+inner, func(_ int, inner *colly.HTMLElement) {
						if title := inner.Attr("title"); title != "" {
							resultChan <- strings.TrimSpace(title)
						} else {
							resultChan <- strings.TrimSpace(inner.Text)
						}
						logging.PrintI("Found content: '%s'", inner.Text)
					})
				}
			})
		}
	}
	return resultChan
}

// looksLikeDate validates if the text appears to be a date
func looksLikeDate(text string) bool {
	text = strings.TrimSpace(strings.ToLower(text))

	// Common date patterns
	datePatterns := []string{
		`\d{4}-\d{2}-\d{2}`,       // YYYY-MM-DD
		`\d{1,2}/\d{1,2}/\d{2,4}`, // M/D/YY or MM/DD/YYYY
		`(?i)(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+\d{1,2},?\s+\d{4}`, // Month DD, YYYY
	}

	for _, pattern := range datePatterns {
		matched, err := regexp.MatchString(pattern, text)
		if err == nil && matched {
			return true
		}
	}

	// Additional date indicators
	dateIndicators := []string{"uploaded", "published", "created", "date:", "on"}
	for _, indicator := range dateIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}

	return false
}
