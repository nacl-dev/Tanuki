package downloader

import (
	"encoding/json"
	"strings"
)

func NormalizeDownloadAutoTags(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func EncodeDownloadAutoTags(values []string) (*json.RawMessage, error) {
	normalized := NormalizeDownloadAutoTags(values)
	if len(normalized) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(body)
	return &raw, nil
}

func DecodeDownloadAutoTags(raw *json.RawMessage) []string {
	if raw == nil || len(*raw) == 0 {
		return nil
	}

	var values []string
	if err := json.Unmarshal(*raw, &values); err != nil {
		return nil
	}
	return NormalizeDownloadAutoTags(values)
}
