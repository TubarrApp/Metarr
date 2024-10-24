package models

import "Metarr/internal/enums"

type MetaReplaceSuffix struct {
	Field       string
	Suffix      string
	Replacement string
}

type MetaReplacePrefix struct {
	Field       string
	Prefix      string
	Replacement string
}

type MetaNewField struct {
	Field string
	Value string
}

type FilenameDatePrefix struct {
	YearLength  int
	MonthLength int
	DayLength   int
	Order       enums.FilenameDateFormat
}

type FilenameReplaceSuffix struct {
	Suffix      string
	Replacement string
}
