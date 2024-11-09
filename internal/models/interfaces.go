package models

import "os"

// Metadata read/write interface
type JSONFileRW interface {
	DecodeJSON(file *os.File) (map[string]interface{}, error)
	RefreshJSON() (map[string]interface{}, error)
	WriteJSON(fieldMap map[string]*string) (map[string]interface{}, error)
	MakeJSONEdits(file *os.File, fd *FileData) (bool, error)
	JSONDateTagEdits(file *os.File, fd *FileData) (edited bool, err error)
}

// Metadata read/write interface
type NFOFileRW interface {
	DecodeMetadata(file *os.File) (*NFOData, error)
	RefreshMetadata() (*NFOData, error)
	MakeMetaEdits(data string, file *os.File, fd *FileData) (bool, error)
}
