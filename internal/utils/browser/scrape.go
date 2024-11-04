package utils

import (
	consts "Metarr/internal/domain/constants"
	enums "Metarr/internal/domain/enums"
	presets "Metarr/internal/utils/browser/presets"
	presetModels "Metarr/internal/utils/browser/presets/models"
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
func ScrapeForMetadata(url string, cookies []*http.Cookie, tag enums.WebClassTags) (string, error) {
	// Define result and error variables
	var (
		result      string
		scrapeError error
	)

	// Initialize the collector
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.MaxDepth(1),
		colly.Async(true),
	)
	c.SetRequestTimeout(15 * time.Second)

	if len(cookies) > 0 {
		c.SetCookies(url, cookies)
	}

	// Define preset scraping rules if the URL matches a known pattern
	switch {
	case strings.Contains(url, "bitchute.com"):

		logging.PrintI("Using bitchute.com preset scraper")
		setupPresetScraping(c, tag, presets.BitchuteComRules, &result, url)

	case strings.Contains(url, "censored.tv"):

		logging.PrintI("Using censored.tv preset scraper")
		if tag == enums.WEBCLASS_CREDITS {
			return presets.CensoredTvChannelName(url), nil
		}
		setupPresetScraping(c, tag, presets.CensoredTvRules, &result, url)

	case strings.Contains(url, "rumble.com"):

		logging.PrintI("Using rumble.com preset scraper")
		setupPresetScraping(c, tag, presets.RumbleComRules, &result, url)

	case strings.Contains(url, "odysee.com"):

		logging.PrintI("Using odysee.com preset scraper")
		setupPresetScraping(c, tag, presets.OdyseeComRules, &result, url)

	default:
		// Regular scraping using predefined tags for non-preset sites
		setupGenericScraping(c, tag, &result, url)
	}

	// Error handler
	c.OnError(func(r *colly.Response, err error) {
		scrapeError = fmt.Errorf("failed to scrape %s: %v", r.Request.URL, err)
	})

	// Attempt visit and wait for async scraping
	if err := c.Visit(url); err != nil {
		return "", fmt.Errorf("unable to visit given web page")
	}
	c.Wait()

	if scrapeError != nil {
		return "", scrapeError
	}
	return result, nil
}

// setupPresetScraping applies specific scraping rules for known sites
func setupPresetScraping(c *colly.Collector, tag enums.WebClassTags, rules map[enums.WebClassTags][]presetModels.SelectorRule, result *string, url string) {
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
				if len(rule.JsonPath) > 0 {
					if jsonVal, err := jsonExtractor([]byte(h.Text), rule.JsonPath); err == nil {
						value = jsonVal
					}
				} else if rule.Attr != "" {
					value = h.Attr(rule.Attr)
				} else {
					value = h.Text
				}

				if value != "" {
					logging.PrintS(0, "Grabbed value '%s' for URL '%s' using preset scraper", value, url)
					*result = rule.Process(value)
				}
			})
		}
	}
}

// setupGenericScraping defines a generic scraping approach for non-preset sites
func setupGenericScraping(c *colly.Collector, tag enums.WebClassTags, result *string, url string) {
	if result == nil {
		return
	}

	var tags []string

	// Determine the appropriate tags based on the metadata being fetched
	switch tag {
	case enums.WEBCLASS_DATE:
		tags = consts.WebDateTags[:]
	case enums.WEBCLASS_DESCRIPTION:
		tags = consts.WebDescriptionTags[:]
	case enums.WEBCLASS_CREDITS:
		tags = consts.WebCreditsTags[:]
	case enums.WEBCLASS_TITLE:
		tags = consts.WebTitleTags[:]
	default:
		return
	}

	// Set up the HTML scraper for each tag
	c.OnHTML("*", func(e *colly.HTMLElement) {
		if *result != "" {
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

				*result = text
				logging.PrintI("Found '%s' in element with class '%s' and id '%s' for URL '%s'",
					result, classAttr, idAttr, url)
				return
			}
		}
	})
}

// jsonExtractor helps extract values from nested JSON structures
func jsonExtractor(data []byte, path []string) (string, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	current := result
	for _, key := range path[:len(path)-1] {
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return "", fmt.Errorf("invalid JSON path at %s", key)
		}
	}
	if val, ok := current[path[len(path)-1]].(string); ok {
		return val, nil
	}
	return "", fmt.Errorf("value at path is not a string")
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
