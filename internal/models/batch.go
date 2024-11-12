package models

type Batch struct {
	Video      string
	Json       string
	IsDirs     bool
	SkipVideos bool

	Core Core
}
