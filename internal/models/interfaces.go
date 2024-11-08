package models

import "os"

// Metadata read/write interface
type JSONFileRW interface {
	DecodeMetadata(file *os.File) (map[string]interface{}, error)
	RefreshMetadata() (map[string]interface{}, error)
	WriteMetadata(fieldMap map[string]*string) (map[string]interface{}, error)
	MakeMetaEdits(data map[string]interface{}, file *os.File, fd *FileData) (bool, error)
}

// Metadata read/write interface
type NFOFileRW interface {
	DecodeMetadata(file *os.File) (*NFOData, error)
	RefreshMetadata() (*NFOData, error)
	MakeMetaEdits(data string, file *os.File, fd *FileData) (bool, error)
}
