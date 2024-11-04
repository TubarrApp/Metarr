package models

import "regexp"

type ContractionPattern struct {
	Regexp      *regexp.Regexp
	Replacement string
}
