package downloader

import "strings"

// ParseEngine selects the most appropriate Engine for a given URL from a list.
// Engines are tried in order; the first match wins.
func ParseEngine(engines []Engine, rawURL string) Engine {
	lower := strings.ToLower(rawURL)
	_ = lower

	for _, e := range engines {
		if e.CanHandle(rawURL) {
			return e
		}
	}
	return nil
}
