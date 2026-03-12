package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nacl-dev/tanuki/internal/importmeta"
	"github.com/nacl-dev/tanuki/internal/models"
	"github.com/nacl-dev/tanuki/internal/scanner"
)

var organizedFolders = map[models.MediaType]string{
	models.MediaTypeVideo:     filepath.Join("Video", "3D (Real)"),
	models.MediaTypeImage:     filepath.Join("Image", "Random"),
	models.MediaTypeManga:     filepath.Join("Comics", "Manga"),
	models.MediaTypeComic:     filepath.Join("Comics", "Manga"),
	models.MediaTypeDoujinshi: filepath.Join("Comics", "Doujins"),
}

func organizeDownloadedFiles(stagingDir, targetRoot string) ([]string, error) {
	moved := make([]string, 0, 4)
	files := make([]organizedMediaCandidate, 0, 8)

	err := filepath.WalkDir(stagingDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isCompanionFile(path) {
			return nil
		}

		mediaType, ok := scanner.MediaTypeForExtension(filepath.Ext(d.Name()))
		if !ok {
			return nil
		}

		relPath, err := filepath.Rel(stagingDir, path)
		if err != nil {
			return err
		}
		files = append(files, organizedMediaCandidate{
			SourcePath: path,
			FileName:   d.Name(),
			MediaType:  mediaType,
			RelDir:     filepath.Dir(relPath),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	groupedMetadata := inferGroupedImportMetadata(files)
	for _, file := range files {
		externalMetadata := readExternalImportMetadata(file.SourcePath)
		targetDirName := classifyDownloadedTarget(stagingDir, file.SourcePath, file.FileName, file.MediaType)
		targetDir := filepath.Join(targetRoot, targetDirName)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", targetDir, err)
		}

		targetPath := uniqueOrganizedPath(targetDir, file.FileName)
		if err := moveWithCompanions(file.SourcePath, targetPath); err != nil {
			return nil, err
		}
		if externalMetadata != nil {
			if err := mergeImportMetadata(targetPath, *externalMetadata); err != nil {
				return nil, fmt.Errorf("write external metadata for %s: %w", targetPath, err)
			}
		}
		if metadata, ok := groupedMetadata[file.SourcePath]; ok {
			if err := mergeImportMetadata(targetPath, metadata); err != nil {
				return nil, fmt.Errorf("write metadata for %s: %w", targetPath, err)
			}
		}
		moved = append(moved, targetPath)
	}

	return moved, nil
}

func isCompanionFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".info.json") || strings.HasSuffix(lower, ".tanuki.json")
}

func uniqueOrganizedPath(dir, name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	ext := filepath.Ext(name)
	candidate := filepath.Join(dir, name)
	idx := 1
	for {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, idx, ext))
		idx++
	}
}

func moveWithCompanions(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return err
	}

	for _, suffix := range []string{".info.json", ".tanuki.json"} {
		companionSrc := src + suffix
		if _, err := os.Stat(companionSrc); err != nil {
			continue
		}
		if err := os.Rename(companionSrc, dst+suffix); err != nil {
			return err
		}
	}
	return nil
}

func classifyDownloadedTarget(stagingDir, fullPath, fileName string, mediaType models.MediaType) string {
	defaultDir := organizedFolders[mediaType]
	lowerName := strings.ToLower(fileName)
	lowerPath := strings.ToLower(fullPath)
	if meta := readImportMetadata(fullPath); meta != nil {
		lowerPath += " " + strings.ToLower(meta.SourceURL) + " " + strings.ToLower(strings.Join(meta.Tags, " "))
	}

	switch mediaType {
	case models.MediaTypeVideo:
		if looksLike2DDownloadedVideo(lowerName, lowerPath) {
			return filepath.Join("Video", "2D (Hentai)")
		}
		if studio := deriveDownloadedStudio(stagingDir, fullPath); studio != "" {
			return filepath.Join("Video", "3D (Real)", studio)
		}
		return filepath.Join("Video", "3D (Real)")
	case models.MediaTypeImage:
		if strings.EqualFold(filepath.Ext(fileName), ".gif") {
			return filepath.Join("Image", "GIFs")
		}
		if strings.Contains(lowerName, "cg") || strings.Contains(lowerPath, "cg") {
			return filepath.Join("Image", "CG Sets")
		}
		return filepath.Join("Image", "Random")
	case models.MediaTypeDoujinshi:
		return filepath.Join("Comics", "Doujins")
	case models.MediaTypeManga, models.MediaTypeComic:
		return filepath.Join("Comics", "Manga")
	default:
		return defaultDir
	}
}

func looksLike2DDownloadedVideo(lowerName, lowerPath string) bool {
	for _, marker := range []string{
		"hentai", "anime", "ova", "doujin", "2d", "animated", "uncensored", "subbed",
	} {
		if strings.Contains(lowerName, marker) || strings.Contains(lowerPath, marker) {
			return true
		}
	}
	return false
}

func deriveDownloadedStudio(stagingRoot, fullPath string) string {
	rel, err := filepath.Rel(stagingRoot, fullPath)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 {
		return ""
	}
	candidate := strings.TrimSpace(parts[0])
	if candidate == "" {
		return ""
	}
	lower := strings.ToLower(candidate)
	for _, generic := range []string{"downloads", "download", "video", "videos", "3d", "new"} {
		if lower == generic {
			return ""
		}
	}
	return sanitizeDownloadedFolder(candidate)
}

func sanitizeDownloadedFolder(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"<", "",
		">", "",
		":", " -",
		"\"", "",
		"/", "-",
		"\\", "-",
		"|", "-",
		"?", "",
		"*", "",
	)
	name = replacer.Replace(name)
	name = strings.Join(strings.Fields(name), " ")
	name = strings.Trim(name, ". ")
	return name
}

func readImportMetadata(mediaPath string) *models.ImportMetadata {
	metadata, err := importmeta.LoadMedia(mediaPath)
	if err != nil {
		return nil
	}
	return metadata
}

func readExternalImportMetadata(mediaPath string) *models.ImportMetadata {
	metadata, recognized, err := importmeta.LoadCompanion(mediaPath + ".json")
	if err != nil || !recognized {
		return nil
	}
	return metadata
}
