package models

import "metarr/internal/domain/enums"

// FilenameOps contains maps related to filename renaming operations.
type FilenameOps struct {
	DateTag         FOpDateTag
	DeleteDateTags  FOpDeleteDateTag
	Set             FOpSet
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
	fo.DateTag.DateFormat = enums.DateFmtSkip        // Zero value
	fo.DeleteDateTags.DateFormat = enums.DateFmtSkip // Zero value

	return fo
}

// EnsureFilenameOps initializes nil filename operation structure values.
func (fd *FileData) EnsureFilenameOps() {
	if fd.FilenameOps == nil {
		fd.FilenameOps = NewFilenameOps()
		return
	}
	if fd.FilenameOps.Appends == nil {
		fd.FilenameOps.Appends = []FOpAppend{}
	}
	if fd.FilenameOps.Prefixes == nil {
		fd.FilenameOps.Prefixes = []FOpPrefix{}
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
	if fd.FilenameOps.ReplaceSuffixes == nil {
		fd.FilenameOps.ReplaceSuffixes = []FOpReplaceSuffix{}
	}
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

// FOpSet can be used to set filenames. Before writing final name changes, check for duplicate filenames and use ++.
type FOpSet struct {
	IsSet bool
	Value string
}
