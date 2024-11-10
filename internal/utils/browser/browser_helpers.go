package utils

import (
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ExtractDomainName extracts various forms of a domain from a URL
func ExtractDomainName(u string) (withProtocol, noProtocol, withProtocolAndPort, noProtocolWithPort string) {
	const (
		https = "https://"
		http  = "http://"
	)

	var (
		proto  string
		port   string
		parseU string
	)

	// Detect and remove protocol if present
	switch {
	case strings.HasPrefix(u, https):
		u = strings.TrimPrefix(u, https)
		proto = https
	case strings.HasPrefix(u, http):
		u = strings.TrimPrefix(u, http)
		proto = http
	}

	// Extract port if present and remove from main URL
	if colIdx := strings.Index(u, ":"); colIdx != -1 {
		portPart := u[colIdx:]
		port, _, _ = strings.Cut(portPart, "/")
		u = u[:colIdx]
	}

	// Prepare URL for parsing
	if proto == "" {
		parseU = https + u
	} else {
		parseU = proto + u
	}

	// Parse the URL
	parsedURL, err := url.Parse(parseU)
	if err != nil {
		return proto + u, u, proto + u + port, u + port
	}

	// Get the host and extract domain
	host := parsedURL.Hostname()
	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return proto + u, u, proto + u + port, u + port
	}

	// Return four variations of the domain
	return (proto + domain), domain, (proto + domain + port), (domain + port)
}
