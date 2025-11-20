// Package browser performs web-related operations like grabbing metadata from a video URL page.
package browser

import (
	"encoding/json"
	"fmt"
	"metarr/internal/domain/consts"
	"metarr/internal/domain/enums"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"metarr/internal/utils/browser/browsepreset"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

const (
	maxRetries = 3
	retryDelay = 5 * time.Second
)

// ScrapeMeta gets cookies for a given URL and returns a grabbed string.
func ScrapeMeta(w *models.MetadataWebData, find enums.WebClassTags) string {

	var (
		err  error
		data string
	)

	w.Cookies, err = getBrowserCookies(w.WebpageURL)
	if err != nil {
		logger.Pl.E("Was unable to grab browser cookies: %v", err)
	}
	for _, try := range w.TryURLs {
		data, err = scrape(try, w.Cookies, find, false)
		if err != nil {
			logger.Pl.E("Failed to scrape %q for requested metadata: %v", try, err)
		} else {
			break
		}
	}
	return data
}

// scrape handles scrape attempts.
func scrape(url string, cookies []*http.Cookie, tag enums.WebClassTags, skipPresets bool) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := attemptScrape(url, cookies, tag, skipPresets)
		if err == nil {
			return result, nil
		}

		lastErr = err
		logger.Pl.E("Scrape attempt %d/%d failed for %s: %v",
			attempt, maxRetries, url, err)

		if attempt < maxRetries {
			logger.Pl.I("Waiting %v before retry...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	return "", fmt.Errorf("all %d scrape attempts failed for %s: %w",
		maxRetries, url, lastErr)
}

// attemptScrape searches relevant URLs to try and fill missing metadata.
func attemptScrape(url string, cookies []*http.Cookie, tag enums.WebClassTags, skipPresets bool) (string, error) {

	var (
		result      string
		scrapeError error
		custom      bool
	)

	// Initialize the collector.
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.MaxDepth(1),
		colly.Async(true),
	)
	c.SetRequestTimeout(15 * time.Second)

	if len(cookies) > 0 {
		if err := c.SetCookies(url, cookies); err != nil {
			return "", fmt.Errorf("failed to set cookies for URL %q: %w", url, err)
		}
	}

	// Define preset scraping rules if the URL matches a known pattern.
	switch {
	case strings.Contains(url, "bitchute.com") && !skipPresets:

		custom = true
		logger.Pl.I("Using bitchute.com preset scraper")
		setupPresetScraping(c, tag, browsepreset.BitchuteComRules, &result, url)

	case strings.Contains(url, "censored.tv") && !skipPresets:

		custom = true
		logger.Pl.I("Using censored.tv preset scraper")
		if tag == enums.WebclassCredits {
			return browsepreset.CensoredTvChannelName(url), nil
		}
		setupPresetScraping(c, tag, browsepreset.CensoredTvRules, &result, url)

	case strings.Contains(url, "rumble.com") && !skipPresets:

		custom = true
		logger.Pl.I("Using rumble.com preset scraper")
		setupPresetScraping(c, tag, browsepreset.RumbleComRules, &result, url)

	case strings.Contains(url, "odysee.com") && !skipPresets:

		custom = true
		logger.Pl.I("Using odysee.com preset scraper")
		setupPresetScraping(c, tag, browsepreset.OdyseeComRules, &result, url)

	default:
		logger.Pl.I("Generic scrape attempt...")
		setupGenericScraping(c, tag, &result, url)
	}

	// Error handler.
	c.OnError(func(r *colly.Response, err error) {
		scrapeError = fmt.Errorf("failed to scrape %s: %w", r.Request.URL, err)
	})

	// Attempt visit and wait for async scraping.
	if err := c.Visit(url); err != nil {
		return "", fmt.Errorf("unable to visit web page %q", url)
	}
	c.Wait()

	if scrapeError != nil {
		switch result {
		case "":
			return "", scrapeError
		default:
			logger.Pl.E("Error during scrape (%v) but got result anyway. Returning result %q...", scrapeError, result)
			return result, nil
		}
	}

	// If custom preset was used and failed, try again with default.
	if result == "" && custom {
		return scrape(url, cookies, tag, true)
	}

	return result, nil
}

// setupPresetScraping applies specific scraping rules for known sites.
func setupPresetScraping(c *colly.Collector, tag enums.WebClassTags, rules map[enums.WebClassTags][]models.SelectorRule, result *string, url string) {
	if result == nil {
		return
	}
	if ruleSet, exists := rules[tag]; exists {
		for _, rule := range ruleSet {
			c.OnHTML(rule.Selector, func(h *colly.HTMLElement) {
				if *result != "" {
					return
				}

				var value string
				switch {
				case len(rule.JSONPath) > 0:
					if jsonVal, err := jsonExtractor([]byte(h.Text), rule.JSONPath); err == nil {
						value = jsonVal
					}

				case rule.Attr != "":
					value = h.Attr(rule.Attr)

				default:
					value = h.Text
				}

				if value != "" {
					logger.Pl.S("Grabbed value %q for URL %q using preset scraper", value, url)
					*result = rule.Process(value)
				}
			})
		}
	}
}

// setupGenericScraping defines a generic scraping approach for non-preset sites.
func setupGenericScraping(c *colly.Collector, tag enums.WebClassTags, result *string, url string) {
	if result == nil {
		return
	}

	var tags []string

	// Determine the appropriate tags based on the metadata being fetched.
	switch tag {
	case enums.WebclassDate:
		tags = consts.WebDateTags[:]
	case enums.WebclassDescription:
		tags = consts.WebDescriptionTags[:]
	case enums.WebclassCredits:
		tags = consts.WebCreditsTags[:]
	case enums.WebclassTitle:
		tags = consts.WebTitleTags[:]
	default:
		return
	}

	// Set up the HTML scraper for each tag.
	c.OnHTML("*", func(e *colly.HTMLElement) {
		if *result != "" {
			return
		}

		classAttr := strings.ToLower(e.Attr("class"))
		idAttr := strings.ToLower(e.Attr("id"))
		text := strings.TrimSpace(e.Text)

		if classAttr != "" {
			logger.Pl.D(2, "Checking element with class: %q", classAttr)
		}

		for _, t := range tags {
			if (e.Name == "p" && strings.Contains(idAttr, t)) ||
				strings.Contains(classAttr, t) ||
				strings.Contains(idAttr, t) {

				if tag == enums.WebclassDate && !looksLikeDate(text) {
					continue
				}

				*result = text
				logger.Pl.I("Found %q in element with class %q and id %q for URL %q",
					*result, classAttr, idAttr, url)
				return
			}
		}
	})
}

// jsonExtractor helps extract values from nested JSON structures.
func jsonExtractor(data []byte, path []string) (string, error) {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	current := result
	for _, key := range path[:len(path)-1] {
		if next, ok := current[key].(map[string]any); ok {
			current = next
		} else {
			return "", fmt.Errorf("invalid JSON path at %s", key)
		}
	}
	if val, ok := current[path[len(path)-1]].(string); ok {
		return val, nil
	}
	return "", fmt.Errorf("value at path %v is not a string, is type %T", path, path)
}

// looksLikeDate validates if the text appears to be a date.
func looksLikeDate(text string) bool {
	text = strings.TrimSpace(strings.ToLower(text))

	// Common date patterns
	datePatterns := []string{
		`\d{4}-\d{2}-\d{2}`,       // YYYY-MM-DD.
		`\d{1,2}/\d{1,2}/\d{2,4}`, // M/D/YY or MM/DD/YYYY.
		`(?i)(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+\d{1,2},?\s+\d{4}`, // Month DD, YYYY.
	}

	for _, pattern := range datePatterns {
		matched, err := regexp.MatchString(pattern, text)
		if err == nil && matched {
			return true
		}
	}

	// Additional date indicators.
	dateIndicators := []string{"uploaded", "published", "created", "date:", "on"}
	for _, indicator := range dateIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}

	return false
}
