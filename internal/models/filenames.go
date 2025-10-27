package models

import "metarr/internal/domain/enums"

// FilenameOps contains maps related to filename renaming operations.
type FilenameOps struct {
	DateTag         FOpDateTag
	DeleteDateTags  FOpDeleteDateTag
	Appends         []FOpAppend
	Prefixes        []FOpPrefix
	Replaces        []FOpReplace
	ReplaceSuffixes []FOpReplaceSuffix
	ReplacePrefixes []FOpReplacePrefix
}

// NewFilenameOps creates a new FilenameOps with initialized slices.
func NewFilenameOps() *FilenameOps {
	fo := &FilenameOps{
		Appends:         make([]FOpAppend, 0),
		Prefixes:        make([]FOpPrefix, 0),
		Replaces:        make([]FOpReplace, 0),
		ReplaceSuffixes: make([]FOpReplaceSuffix, 0),
		ReplacePrefixes: make([]FOpReplacePrefix, 0),
	}
	fo.DateTag.DateFormat = enums.DateFmtSkip
	fo.DeleteDateTags.DateFormat = enums.DateFmtSkip

	return fo
}

// EnsureFilenameOps returns the provided FilenameOps or creates a new one if nil.
func EnsureFilenameOps(fOps *FilenameOps) *FilenameOps {
	if fOps == nil {
		return NewFilenameOps()
	}
	return fOps
}

// FOpAppend is the value to append onto a filename.
type FOpAppend struct {
	Value string
}

// FOpPrefix is the value to prefix on a filename.
type FOpPrefix struct {
	Value string
}

// FOpDateTag is the format and location to enter a date tag onto the filename.
type FOpDateTag struct {
	Loc        enums.DateTagLocation
	DateFormat enums.DateFormat
}

// FOpDeleteDateTag is the format and location from which to delete a date tag from the filename.
type FOpDeleteDateTag struct {
	Loc        enums.DateTagLocation
	DateFormat enums.DateFormat
}

// FOpReplace is the string to find and what to replace those strings with, in a filename.
type FOpReplace struct {
	FindString  string
	Replacement string
}

// FOpReplaceSuffix is the suffix to trim and what to replace it with.
type FOpReplaceSuffix struct {
	Suffix      string
	Replacement string
}

// FOpReplacePrefix is the prefix to trim and what to replace it with.
type FOpReplacePrefix struct {
	Prefix      string
	Replacement string
}
