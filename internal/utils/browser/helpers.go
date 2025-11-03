package browser

import (
	logging "metarr/internal/utils/logging"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ExtractDomainName extracts various forms of a domain from a URL.
func ExtractDomainName(u string) (withProtocol, noProtocol, withProtocolAndPort, noProtocolWithPort string) {
	const (
		https = "https://"
		http  = "http://"
	)

	var (
		proto string
		port  string
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
		parts := strings.SplitN(portPart, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			port = parts[0]
		}
		u = u[:colIdx]
	}

	// Prepare URL for parsing
	parseProto := proto
	if parseProto == "" {
		parseProto = https
	}

	// Parse the URL
	parsedURL, err := url.Parse(parseProto + u)
	if err != nil {
		return makeURLStrings(proto, u, port)
	}

	// Get the host and extract domain
	host := parsedURL.Hostname()
	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return makeURLStrings(proto, u, port)
	}

	return makeURLStrings(proto, domain, port)
}

// Private /////////////////////////////////////////////

// makeURLStrings builds the URL strings using strings.Builder.
func makeURLStrings(proto, domain, port string) (withProtocol, noProtocol, withProtocolAndPort, noProtocolWithPort string) {
	var b strings.Builder

	// Calculate maximum capacity needed
	maxLen := len(proto) + len(domain) + len(port)

	// Build withProtocol
	b.Grow(maxLen)
	b.WriteString(proto)
	b.WriteString(domain)
	withProtocol = b.String()

	// Build withProtocolAndPort
	b.WriteString(port)
	withProtocolAndPort = b.String()

	// Build noProtocol
	b.Reset()
	b.Grow(maxLen)
	b.WriteString(domain)
	noProtocol = b.String()

	// Build noProtocolWithPort
	b.WriteString(port)
	noProtocolWithPort = b.String()

	logging.D(1, "Made URL strings:\n\nWith protocol: %q\nNo protocol: %q\nProtocol + port: %q\nNo protocol + port: %q\n",
		withProtocol,
		noProtocol,
		withProtocolAndPort,
		noProtocolWithPort)

	return
}
