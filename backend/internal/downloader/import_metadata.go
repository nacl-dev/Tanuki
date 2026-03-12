package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
)

var (
	workKeywordSuffixRe      = regexp.MustCompile(`(?i)^(.*?)\s+(?:episode|ep|part|chapter|ch|volume|vol|scene|issue|book)\s*#?0*([1-9]\d{0,3})$`)
	workSeparatorSuffixRe    = regexp.MustCompile(`(?i)^(.*?)\s*(?:[-_:]|#)\s*0*([1-9]\d{0,3})$`)
	workLeadingZeroSuffixRe  = regexp.MustCompile(`(?i)^(.*?)\s+0+([1-9]\d{0,3})$`)
	workKeywordEmbeddedRe    = regexp.MustCompile(`(?i)^(.*?)\s+(?:episode|ep|part|chapter|ch|scene|issue|book)\.?\s*#?0*([1-9]\d{0,3})(?:\b.*)?$`)
	workVolumeChapterRe      = regexp.MustCompile(`(?i)^(.*?)\s+(?:(?:volume|vol|v)\.?\s*0*([1-9]\d{0,2})\s*(?:[-_:]|/)?\s*)?(?:chapter|ch|c)\.?\s*#?0*([1-9]\d{0,3})(?:\b.*)?$`)
	workVolumeOnlyEmbeddedRe = regexp.MustCompile(`(?i)^(.*?)\s+(?:volume|vol|v)\.?\s*0*([1-9]\d{0,2})(?:\b.*)?$`)
	workSeasonEpisodeRe      = regexp.MustCompile(`(?i)^(.*?)\s+S(?:eason)?\s*0*([1-9]\d{0,2})\s*[-_. ]*E(?:pisode)?\s*0*([1-9]\d{0,3})(?:\b.*)?$`)
	workEpisodeVersionRe     = regexp.MustCompile(`(?i)^(.*?)\s+0*([1-9]\d{0,3})v[1-9]\d*(?:\b.*)?$`)
	workTrailingNumberRe     = regexp.MustCompile(`(?i)(\d{1,4})$`)
	workTitlePrefixCleanupRe = regexp.MustCompile(`[0-9\s._\-:#]+$`)
	workBracketPrefixRe      = regexp.MustCompile(`^(?:\[[^\]]+\]\s*|\([^\)]+\)\s*)+`)
	workBracketSuffixRe      = regexp.MustCompile(`(?:\s+\[[^\]]+\])+$`)
	workTrailingParodyRe     = regexp.MustCompile(`\s+\([^\)]+\)$`)
	lettersRe                = regexp.MustCompile(`[A-Za-z]`)
)

func writeImportMetadata(mediaPath string, metadata models.ImportMetadata) error {
	metadata = normalizeImportMetadata(mediaPath, metadata)
	if importMetadataEmpty(metadata) {
		return nil
	}

	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mediaPath+".tanuki.json", body, 0o644)
}

func mergeImportMetadata(mediaPath string, updates models.ImportMetadata) error {
	metadata := readImportMetadata(mediaPath)
	if metadata == nil {
		metadata = &models.ImportMetadata{}
	}

	if title := strings.TrimSpace(updates.Title); title != "" && strings.TrimSpace(metadata.Title) == "" {
		metadata.Title = title
	}
	if workTitle := strings.TrimSpace(updates.WorkTitle); workTitle != "" && strings.TrimSpace(metadata.WorkTitle) == "" {
		metadata.WorkTitle = workTitle
	}
	if updates.WorkIndex > 0 && metadata.WorkIndex == 0 {
		metadata.WorkIndex = updates.WorkIndex
	}
	if sourceURL := strings.TrimSpace(updates.SourceURL); sourceURL != "" && strings.TrimSpace(metadata.SourceURL) == "" {
		metadata.SourceURL = sourceURL
	}
	if posterURL := strings.TrimSpace(updates.PosterURL); posterURL != "" && strings.TrimSpace(metadata.PosterURL) == "" {
		metadata.PosterURL = posterURL
	}
	if len(updates.Tags) > 0 {
		metadata.Tags = mergeImportTags(metadata.Tags, updates.Tags)
	}

	return writeImportMetadata(mediaPath, *metadata)
}

func normalizeImportMetadata(mediaPath string, metadata models.ImportMetadata) models.ImportMetadata {
	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.WorkTitle = cleanWorkTitleCandidate(metadata.WorkTitle)
	if metadata.WorkIndex < 0 {
		metadata.WorkIndex = 0
	}
	metadata.SourceURL = strings.TrimSpace(metadata.SourceURL)
	metadata.PosterURL = strings.TrimSpace(metadata.PosterURL)
	metadata.Tags = compactStrings(metadata.Tags)

	if metadata.WorkTitle == "" || metadata.WorkIndex == 0 {
		if inferredTitle, inferredIndex, ok := inferWorkMetadata(mediaPath, metadata.Title); ok {
			if metadata.WorkTitle == "" {
				metadata.WorkTitle = inferredTitle
			}
			if metadata.WorkIndex == 0 {
				metadata.WorkIndex = inferredIndex
			}
		}
	}
	if metadata.WorkTitle == "" {
		if inferredTitle := inferHentaiStyleWorkTitle(metadata.Title); inferredTitle != "" {
			metadata.WorkTitle = inferredTitle
		} else if inferredTitle := inferHentaiStyleWorkTitle(strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))); inferredTitle != "" {
			metadata.WorkTitle = inferredTitle
		}
	}

	return metadata
}

func importMetadataEmpty(metadata models.ImportMetadata) bool {
	return strings.TrimSpace(metadata.Title) == "" &&
		strings.TrimSpace(metadata.WorkTitle) == "" &&
		metadata.WorkIndex == 0 &&
		strings.TrimSpace(metadata.SourceURL) == "" &&
		strings.TrimSpace(metadata.PosterURL) == "" &&
		len(metadata.Tags) == 0
}

func mergeImportTags(existing, updates []string) []string {
	values := make([]string, 0, len(existing)+len(updates))
	values = append(values, existing...)
	values = append(values, updates...)
	return compactStrings(values)
}

func inferWorkMetadata(mediaPath, title string) (string, int, bool) {
	title = strings.TrimSpace(title)
	if inferredTitle, inferredIndex, ok := inferWorkMetadataFromName(title); ok {
		return inferredTitle, inferredIndex, true
	}

	baseName := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	return inferWorkMetadataFromName(baseName)
}

func inferWorkMetadataFromName(name string) (string, int, bool) {
	name = normalizeWorkName(name)
	if name == "" {
		return "", 0, false
	}

	if match := workSeasonEpisodeRe.FindStringSubmatch(name); len(match) == 4 {
		workTitle := cleanWorkTitleCandidate(match[1])
		workIndex, err := strconv.Atoi(match[3])
		if err == nil && workIndex > 0 && workTitle != "" {
			return workTitle, workIndex, true
		}
	}

	if match := workVolumeChapterRe.FindStringSubmatch(name); len(match) == 4 {
		workTitle := cleanWorkTitleCandidate(match[1])
		workIndex, err := strconv.Atoi(match[3])
		if err == nil && workIndex > 0 && workTitle != "" {
			return workTitle, workIndex, true
		}
	}

	for _, re := range []*regexp.Regexp{
		workKeywordSuffixRe,
		workSeparatorSuffixRe,
		workLeadingZeroSuffixRe,
		workKeywordEmbeddedRe,
		workVolumeOnlyEmbeddedRe,
		workEpisodeVersionRe,
	} {
		match := re.FindStringSubmatch(name)
		if len(match) != 3 {
			continue
		}
		workTitle := cleanWorkTitleCandidate(match[1])
		workIndex, err := strconv.Atoi(match[2])
		if err != nil || workIndex <= 0 || workTitle == "" {
			continue
		}
		return workTitle, workIndex, true
	}

	return "", 0, false
}

func inferExplicitWorkIndex(name string) (int, bool) {
	name = normalizeWorkName(name)
	if name == "" {
		return 0, false
	}
	if _, index, ok := inferWorkMetadataFromName(name); ok {
		return index, true
	}

	match := workTrailingNumberRe.FindStringSubmatch(name)
	if len(match) != 2 {
		return 0, false
	}

	index, err := strconv.Atoi(match[1])
	if err != nil || index <= 0 {
		return 0, false
	}
	return index, true
}

type organizedMediaCandidate struct {
	SourcePath string
	FileName   string
	MediaType  models.MediaType
	RelDir     string
}

func inferGroupedImportMetadata(files []organizedMediaCandidate) map[string]models.ImportMetadata {
	type groupKey struct {
		RelDir    string
		MediaType models.MediaType
	}

	groups := make(map[groupKey][]organizedMediaCandidate)
	for _, file := range files {
		key := groupKey{RelDir: file.RelDir, MediaType: file.MediaType}
		groups[key] = append(groups[key], file)
	}

	metadataByPath := make(map[string]models.ImportMetadata)
	for key, group := range groups {
		if len(group) < 2 {
			continue
		}

		workTitle := deriveGroupedWorkTitle(key.RelDir, group)
		if workTitle == "" {
			continue
		}

		for path, workIndex := range deriveGroupedWorkIndexes(group) {
			metadataByPath[path] = models.ImportMetadata{
				WorkTitle: workTitle,
				WorkIndex: workIndex,
			}
		}
	}

	return metadataByPath
}

func deriveGroupedWorkTitle(relDir string, group []organizedMediaCandidate) string {
	if workTitle := workTitleFromRelativeDir(relDir); workTitle != "" {
		return workTitle
	}

	trimmedTitles := make([]string, 0, len(group))
	for _, file := range group {
		baseName := strings.TrimSuffix(file.FileName, filepath.Ext(file.FileName))
		if workTitle, _, ok := inferWorkMetadataFromName(baseName); ok {
			trimmedTitles = append(trimmedTitles, workTitle)
			continue
		}
		trimmedTitles = append(trimmedTitles, strings.TrimSpace(baseName))
	}

	if allEqualFold(trimmedTitles) {
		return cleanWorkTitleCandidate(trimmedTitles[0])
	}

	return deriveCommonWorkPrefix(trimmedTitles)
}

func deriveGroupedWorkIndexes(group []organizedMediaCandidate) map[string]int {
	indexes := make(map[string]int, len(group))
	explicit := make(map[string]int, len(group))
	seen := make(map[int]struct{}, len(group))
	canUseExplicit := true

	for _, file := range group {
		baseName := strings.TrimSuffix(file.FileName, filepath.Ext(file.FileName))
		workIndex, ok := inferExplicitWorkIndex(baseName)
		if !ok {
			canUseExplicit = false
			break
		}
		if _, duplicate := seen[workIndex]; duplicate {
			canUseExplicit = false
			break
		}
		seen[workIndex] = struct{}{}
		explicit[file.SourcePath] = workIndex
	}
	if canUseExplicit && len(explicit) == len(group) {
		return explicit
	}

	sorted := append([]organizedMediaCandidate(nil), group...)
	sort.SliceStable(sorted, func(i, j int) bool {
		leftIndex, leftOK := inferExplicitWorkIndex(strings.TrimSuffix(sorted[i].FileName, filepath.Ext(sorted[i].FileName)))
		rightIndex, rightOK := inferExplicitWorkIndex(strings.TrimSuffix(sorted[j].FileName, filepath.Ext(sorted[j].FileName)))
		if leftOK && rightOK && leftIndex != rightIndex {
			return leftIndex < rightIndex
		}
		leftName := strings.ToLower(sorted[i].FileName)
		rightName := strings.ToLower(sorted[j].FileName)
		if leftName != rightName {
			return leftName < rightName
		}
		return sorted[i].SourcePath < sorted[j].SourcePath
	})

	for i, file := range sorted {
		indexes[file.SourcePath] = i + 1
	}
	return indexes
}

func workTitleFromRelativeDir(relDir string) string {
	relDir = strings.TrimSpace(relDir)
	if relDir == "" || relDir == "." {
		return ""
	}

	parts := strings.Split(filepath.ToSlash(filepath.Clean(relDir)), "/")
	if len(parts) == 0 {
		return ""
	}

	last := cleanWorkTitleCandidate(parts[len(parts)-1])
	if last == "" {
		return ""
	}
	if len(parts) == 1 || !looksWeakWorkTitle(last) {
		return last
	}

	previous := cleanWorkTitleCandidate(parts[len(parts)-2])
	if previous == "" {
		return last
	}
	return cleanWorkTitleCandidate(previous + " " + last)
}

func looksWeakWorkTitle(title string) bool {
	if title == "" {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(title))
	for _, generic := range []string{"downloads", "download", "images", "image", "gallery", "chapter", "chapters", "new"} {
		if lower == generic {
			return true
		}
	}
	return !lettersRe.MatchString(title)
}

func deriveCommonWorkPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}

	prefix := strings.TrimSpace(values[0])
	for _, value := range values[1:] {
		prefix = sharedPrefix(prefix, strings.TrimSpace(value))
		if prefix == "" {
			return ""
		}
	}

	prefix = workTitlePrefixCleanupRe.ReplaceAllString(prefix, "")
	prefix = cleanWorkTitleCandidate(prefix)
	if len(prefix) < 3 || !lettersRe.MatchString(prefix) {
		return ""
	}
	return prefix
}

func sharedPrefix(left, right string) string {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	limit := len(leftRunes)
	if len(rightRunes) < limit {
		limit = len(rightRunes)
	}

	idx := 0
	for idx < limit {
		if strings.ToLower(string(leftRunes[idx])) != strings.ToLower(string(rightRunes[idx])) {
			break
		}
		idx++
	}
	return string(leftRunes[:idx])
}

func allEqualFold(values []string) bool {
	if len(values) == 0 {
		return false
	}
	first := strings.TrimSpace(values[0])
	if first == "" {
		return false
	}
	for _, value := range values[1:] {
		if !strings.EqualFold(first, strings.TrimSpace(value)) {
			return false
		}
	}
	return true
}

func cleanWorkTitleCandidate(title string) string {
	title = sanitizeArchiveName(title)
	title = strings.Trim(title, "-_:.# ")
	title = strings.TrimSpace(title)
	return title
}

func normalizeWorkName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = workBracketPrefixRe.ReplaceAllString(name, "")
	name = strings.Join(strings.Fields(name), " ")
	return strings.TrimSpace(name)
}

func inferHentaiStyleWorkTitle(name string) string {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return ""
	}

	trimmed := workBracketPrefixRe.ReplaceAllString(raw, "")
	trimmed = workBracketSuffixRe.ReplaceAllString(trimmed, "")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" || trimmed == raw {
		return ""
	}

	if strings.Contains(raw, "[") {
		withoutParody := strings.TrimSpace(workTrailingParodyRe.ReplaceAllString(trimmed, ""))
		if looksStrongWorkTitle(withoutParody) {
			return cleanWorkTitleCandidate(withoutParody)
		}
	}

	if looksStrongWorkTitle(trimmed) {
		return cleanWorkTitleCandidate(trimmed)
	}
	return ""
}

func looksStrongWorkTitle(title string) bool {
	title = cleanWorkTitleCandidate(title)
	if len(title) < 3 {
		return false
	}
	return lettersRe.MatchString(title)
}

func cleanWorkTitle(title string) string {
	return cleanWorkTitleCandidate(title)
}

func zeroPadInt(value, width int) string {
	if width <= 1 {
		return strconv.Itoa(value)
	}
	return fmt.Sprintf("%0*d", width, value)
}

func parsePositiveInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}
