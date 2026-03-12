package downloader

import "strings"

func qualifyTags(namespace string, values []string) []string {
	namespace = strings.TrimSpace(strings.ToLower(namespace))
	if namespace == "" {
		return compactStrings(values)
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, namespace+":"+value)
	}
	return compactStrings(out)
}
