package models

import "os"

// JSONFileRW contains methods to work with JSON metadata/files.
type JSONFileRW interface {
	DecodeJSON(file *os.File) (map[string]any, error)
	RefreshJSON() (map[string]any, error)
	WriteJSON(fieldMap map[string]*string) (map[string]any, error)
	MakeJSONEdits(file *os.File, fd *FileData) (bool, error)
	JSONDateTagEdits(file *os.File, fd *FileData) (edited bool, err error)
}

// NFOFileRW contains methods to work with XML metadata/files.
type NFOFileRW interface {
	DecodeMetadata(file *os.File) (*NFOData, error)
	RefreshMetadata() (*NFOData, error)
	MakeMetaEdits(data string, file *os.File, fd *FileData) (bool, error)
}
