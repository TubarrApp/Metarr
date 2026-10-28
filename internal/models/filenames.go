package models

import (
	"regexp"

	"github.com/TubarrApp/gocommon/sharedenums"
)

// FilenameOps contains maps related to filename renaming operations.
type FilenameOps struct {
	DateTag         FOpDateTag
	DeleteDateTags  FOpDeleteDateTag
	Set             FOpSet
	Prefixes        []FOpPrefix
	Suffixes        []FOpSuffix
	Replaces        []FOpReplace
	ReplaceSuffixes []FOpReplaceSuffix
	ReplacePrefixes []FOpReplacePrefix
	RegexReplaces   []FOpRegexReplace
	Cuts            []TextCut
}

// NewFilenameOps creates a new FilenameOps with initialized slices.
func NewFilenameOps() *FilenameOps {
	fo := &FilenameOps{
		Prefixes:        make([]FOpPrefix, 0),
		Suffixes:        make([]FOpSuffix, 0),
		Replaces:        make([]FOpReplace, 0),
		ReplaceSuffixes: make([]FOpReplaceSuffix, 0),
		ReplacePrefixes: make([]FOpReplacePrefix, 0),
		RegexReplaces:   make([]FOpRegexReplace, 0),
		Cuts:            make([]TextCut, 0),
	}
	fo.DateTag.DateFormat = sharedenums.DateFmtSkip        // Zero value.
	fo.DeleteDateTags.DateFormat = sharedenums.DateFmtSkip // Zero value.

	return fo
}

// EnsureFilenameOps initializes nil filename operation structure values.
func (fd *FileData) EnsureFilenameOps() {
	if fd.FilenameOps == nil {
		fd.FilenameOps = NewFilenameOps()
		return
	}
	if fd.FilenameOps.Prefixes == nil {
		fd.FilenameOps.Prefixes = []FOpPrefix{}
	}
	if fd.FilenameOps.Suffixes == nil {
		fd.FilenameOps.Suffixes = []FOpSuffix{}
	}
	if fd.FilenameOps.Replaces == nil {
		fd.FilenameOps.Replaces = []FOpReplace{}
	}
	if fd.FilenameOps.ReplacePrefixes == nil {
		fd.FilenameOps.ReplacePrefixes = []FOpReplacePrefix{}
	}
	if fd.FilenameOps.ReplaceSuffixes == nil {
		fd.FilenameOps.ReplaceSuffixes = []FOpReplaceSuffix{}
	}
	if fd.FilenameOps.RegexReplaces == nil {
		fd.FilenameOps.RegexReplaces = []FOpRegexReplace{}
	}
	if fd.FilenameOps.Cuts == nil {
		fd.FilenameOps.Cuts = []TextCut{}
	}
}

// FOpPrefix is the value to prefix on a filename.
type FOpPrefix struct {
	Value string
}

// FOpSuffix is the value to suffix on a filename.
type FOpSuffix struct {
	Value string
}

// FOpDateTag is the format and location to enter a date tag onto the filename.
type FOpDateTag struct {
	Loc        sharedenums.DateTagLocation
	DateFormat sharedenums.DateFormat
}

// FOpDeleteDateTag is the format and location from which to delete a date tag from the filename.
type FOpDeleteDateTag struct {
	Loc        sharedenums.DateTagLocation
	DateFormat sharedenums.DateFormat
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

// FOpRegexReplace replaces every match of a pattern in a filename.
type FOpRegexReplace struct {
	Regex       *regexp.Regexp
	Replacement string
}

// FOpSet can be used to set filenames. -- Before writing final name changes, check for duplicate filenames. --
type FOpSet struct {
	IsSet bool
	Value string
}
