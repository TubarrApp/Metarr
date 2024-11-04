package utils

import (
	enums "Metarr/internal/domain/enums"
	logging "Metarr/internal/utils/logging"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gocolly/colly"
)

type selectorRule struct {
	selector string
	attr     string // empty for text content, otherwise attribute name
	process  func(string) string
	jsonPath []string
}

// WebScrapeSwitch checks if a preset scraper exists for this domain, and uses it if possible
func WebScrapeSwitch(url string, cookies []*http.Cookie, fetch enums.WebClassTags) (out string, err error, urlMatched bool) {

	switch {
	case strings.Contains(url, "censored.tv"):

		if fetch == enums.WEBCLASS_CREDITS {
			return censoredTvChannelName(url), nil, true
		}
		rtn, err := presetScrape(url, cookies, fetch, censoredTvRules)
		logging.PrintI("Got scraped content '%s' from '%s'", rtn, url)
		return rtn, err, true

	case strings.Contains(url, "odysee.com"):
		rtn, err := presetScrape(url, cookies, fetch, odyseeComRules)
		logging.PrintI("Got scraped content '%s' from '%s'", rtn, url)
		return rtn, err, true
	}

	return "", nil, false
}

// ScrapeCensoredTvMeta scrapes the web for metadata details for censored.tv video pages
func presetScrape(url string, cookies []*http.Cookie, fetch enums.WebClassTags, ruleMap map[enums.WebClassTags][]selectorRule) (string, error) {

	var (
		result      string
		scrapeError error
	)

	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.MaxDepth(1),
		colly.Async(true),
	)

	if len(cookies) > 0 {
		c.SetCookies(url, cookies)
	}

	// Set up error handler
	c.OnError(func(r *colly.Response, err error) {
		scrapeError = fmt.Errorf("encountered scrape error: %v", r.Request.URL)
	})

	// Set up scraping rules based on tag type
	if rules, exists := ruleMap[fetch]; exists {
		setupScrapeRules(c, &result, rules)
	} else {
		return "", fmt.Errorf("no rules exist for that enum")
	}

	// Visit URL and wait for all scraping processes to finish
	if err := c.Visit(url); err != nil {
		return "", fmt.Errorf("unable to visit given web page")
	}
	c.Wait()

	return result, scrapeError
}

// censoredTvTitle is the title handler
func setupScrapeRules(c *colly.Collector, result *string, rules []selectorRule) {
	if result == nil || c == nil {
		logging.PrintE(0, "string or colly passed in null")
		return
	}

	var complete bool
	for _, rule := range rules {
		c.OnHTML(rule.selector, func(h *colly.HTMLElement) {
			if complete {
				return
			}

			var value string
			if len(rule.jsonPath) > 0 {
				// Handle JSON content
				if jsonVal, err := jsonExtractor([]byte(h.Text), rule.jsonPath); err == nil {
					value = jsonVal
					logging.PrintD(3, "Found JSON value at path %v: '%s'", rule.jsonPath, value)
				}
			} else if rule.attr != "" {
				value = h.Attr(rule.attr)
			} else {
				value = h.Text
			}

			if value != "" {
				*result = rule.process(value)
				logging.PrintD(3, "Found %s: '%s'", rule.selector, *result)
				complete = true
			}
		})
	}
}

// jsonExtractor helps extract values from nested JSON structures
func jsonExtractor(data []byte, path []string) (string, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	current := result
	for i, key := range path[:len(path)-1] {
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return "", fmt.Errorf("invalid JSON path at %s", path[i])
		}
	}

	lastKey := path[len(path)-1]
	if val, ok := current[lastKey].(string); ok {
		return val, nil
	}
	return "", fmt.Errorf("value at path is not a string")
}
