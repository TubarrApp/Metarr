package models

import "encoding/xml"

// NFOData represents the complete NFO file structure.
type NFOData struct {
	XMLName     xml.Name    `xml:"movie"`
	Title       Title       `xml:"title"`
	Plot        string      `xml:"plot"`
	Description string      `xml:"description"`
	Actors      []Person    `xml:"cast>actor"`
	Directors   []string    `xml:"director"`
	Producers   []string    `xml:"producer"`
	Publishers  []string    `xml:"publisher"`
	Writers     []string    `xml:"writer"`
	Studios     []string    `xml:"studio"`
	Year        string      `xml:"year"`
	Premiered   string      `xml:"premiered"`
	ReleaseDate string      `xml:"releasedate"`
	ShowInfo    ShowInfo    `xml:"showinfo"`
	WebpageInfo WebpageInfo `xml:"web"`

	// Movie-level web fields, as written by the NFO builder and used by Kodi.
	// The nested WebpageInfo form above is still read as a fallback.
	URL    string `xml:"url"`
	Thumb  string `xml:"thumb"`
	Poster string `xml:"poster"`
	Fanart string `xml:"fanart"`
}

// Title represents nested title information.
type Title struct {
	Main      string `xml:"main"`
	Original  string `xml:"original"`
	Sort      string `xml:"sort"`
	Sub       string `xml:"sub"`
	PlainText string `xml:",chardata"` // For non-nested titles.
}

// Person represents a credited person with optional role.
type Person struct {
	Name  string `xml:"name"`
	Role  string `xml:"role"`
	Order int    `xml:"order"`
	Thumb string `xml:"thumb"`
}

// ShowInfo represents TV show specific information.
type ShowInfo struct {
	Show         string `xml:"show"`
	SeasonNumber string `xml:"season>number"`
	EpisodeID    string `xml:"episode>number"`
	EpisodeTitle string `xml:"episode>title"`
}

// WebpageInfo represents web page data such as web/thumbnail URLs.
type WebpageInfo struct {
	URL    string `xml:"url"`
	Fanart string `xml:"fanart"`
	Thumb  string `xml:"thumb"`
}
