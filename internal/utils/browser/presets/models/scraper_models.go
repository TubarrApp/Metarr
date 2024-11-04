package utils

type SelectorRule struct {
	Selector string
	Attr     string // empty for text content, otherwise attribute name
	Process  func(string) string
	JsonPath []string
}
