package models

import "regexp"

// ContractionPattern creates a pattern model for regex matching and replacing.
type ContractionPattern struct {
	Regexp      *regexp.Regexp
	Replacement string
}
