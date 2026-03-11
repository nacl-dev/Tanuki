// Package thumbnails generates preview thumbnails for media items.
package thumbnails

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	maxWidth    = 300
	maxHeight   = 300
	jpegQuality = 85
)

// Generator creates thumbnail images for media items.
type Generator struct {
	thumbnailsPath string
	log            *zap.Logger
}

// New returns a Generator that writes thumbnails to thumbnailsPath.
func New(thumbnailsPath string, log *zap.Logger) *Generator {
	return &Generator{thumbnailsPath: thumbnailsPath, log: log}
}

// GenerateForMedia creates a thumbnail for the given media item and returns the
// path where the thumbnail was saved. The caller is responsible for persisting
// the returned path to the database.
func (g *Generator) GenerateForMedia(ctx context.Context, media *models.Media) (string, error) {
	if err := os.MkdirAll(g.thumbnailsPath, 0o755); err != nil {
		return "", fmt.Errorf("create thumbnails dir: %w", err)
	}

	outPath := filepath.Join(g.thumbnailsPath, media.ID+".jpg")

	switch media.Type {
	case models.MediaTypeImage:
		if err := g.generateFromImage(media.FilePath, outPath); err != nil {
			return "", err
		}
	case models.MediaTypeVideo:
		if err := g.generateFromVideo(ctx, media.FilePath, outPath); err != nil {
			return "", err
		}
	case models.MediaTypeManga, models.MediaTypeComic, models.MediaTypeDoujinshi:
		if err := g.generateFromArchive(ctx, media.FilePath, outPath); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported media type: %s", media.Type)
	}

	return outPath, nil
}

// generateFromImage resizes the image at src and saves a JPEG thumbnail to dst.
func (g *Generator) generateFromImage(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open image %s: %w", src, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode image %s: %w", src, err)
	}

	thumb := resizeImage(img, maxWidth, maxHeight)
	return saveJPEG(thumb, dst)
}

// generateFromVideo shells out to ffmpeg to extract a frame at ~10% of duration.
func (g *Generator) generateFromVideo(ctx context.Context, src, dst string) error {
	// Probe duration with ffprobe first; fall back to 10 seconds if unavailable.
	timestamp := probeVideoTimestamp(ctx, src)

	scale := fmt.Sprintf("scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease", maxWidth, maxHeight)
	cmd := exec.CommandContext(ctx,
		"ffmpeg", "-y",
		"-i", src,
		"-ss", timestamp,
		"-vframes", "1",
		"-vf", scale,
		"-q:v", "3",
		dst,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w\n%s", err, string(out))
	}
	return nil
}

// probeVideoTimestamp uses ffprobe to find a timestamp at ~10% of the video duration.
// Returns "00:00:10" as a safe fallback.
func probeVideoTimestamp(ctx context.Context, src string) string {
	cmd := exec.CommandContext(ctx,
		"ffprobe", "-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		src,
	)
	out, err := cmd.Output()
	if err != nil {
		return "00:00:10"
	}
	var dur float64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &dur); err != nil || dur <= 0 {
		return "00:00:10"
	}
	ts := dur * 0.1
	h := int(ts) / 3600
	m := (int(ts) % 3600) / 60
	s := int(ts) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// generateFromArchive opens a ZIP/CBZ/CBR archive and generates a thumbnail from
// the first image file found inside (sorted alphabetically).
func (g *Generator) generateFromArchive(ctx context.Context, src, dst string) error {
	ext := strings.ToLower(filepath.Ext(src))
	switch ext {
	case ".cbr", ".rar":
		return g.generateFromRAR(ctx, src, dst)
	default:
		return g.generateFromZIP(src, dst)
	}
}

// generateFromZIP handles ZIP and CBZ archives.
func (g *Generator) generateFromZIP(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", src, err)
	}
	defer r.Close()

	// Collect image files and sort them.
	var imageFiles []*zip.File
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if isImageExt(filepath.Ext(f.Name)) {
			imageFiles = append(imageFiles, f)
		}
	}
	if len(imageFiles) == 0 {
		return fmt.Errorf("no images found in archive %s", src)
	}
	sort.Slice(imageFiles, func(i, j int) bool {
		return imageFiles[i].Name < imageFiles[j].Name
	})

	rc, err := imageFiles[0].Open()
	if err != nil {
		return fmt.Errorf("open archive entry: %w", err)
	}
	defer rc.Close()

	img, _, err := image.Decode(rc)
	if err != nil {
		return fmt.Errorf("decode archive image: %w", err)
	}

	thumb := resizeImage(img, maxWidth, maxHeight)
	return saveJPEG(thumb, dst)
}

// generateFromRAR shells out to bsdtar to read the first image, then thumbnails it.
func (g *Generator) generateFromRAR(ctx context.Context, src, dst string) error {
	listCmd := exec.CommandContext(ctx, "bsdtar", "-tf", src)
	listOut, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("bsdtar -tf: %w", err)
	}

	var images []string
	for _, line := range strings.Split(string(listOut), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && isImageExt(filepath.Ext(line)) {
			images = append(images, line)
		}
	}
	if len(images) == 0 {
		return fmt.Errorf("no images found in RAR archive %s", src)
	}
	sort.Strings(images)

	extractCmd := exec.CommandContext(ctx, "bsdtar", "-xOf", src, images[0])
	imageBytes, err := extractCmd.Output()
	if err != nil {
		return fmt.Errorf("bsdtar -xOf: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return fmt.Errorf("decode rar image: %w", err)
	}

	thumb := resizeImage(img, maxWidth, maxHeight)
	return saveJPEG(thumb, dst)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// resizeImage proportionally resizes img to fit within maxW × maxH using
// high-quality Lanczos resampling.
func resizeImage(img image.Image, maxW, maxH int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return img
	}

	scale := math.Min(float64(maxW)/float64(srcW), float64(maxH)/float64(srcH))
	if scale >= 1 {
		return img // image is already small enough
	}

	dstW := int(math.Round(float64(srcW) * scale))
	dstH := int(math.Round(float64(srcH) * scale))
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}

// saveJPEG encodes img as a JPEG and writes it to path.
func saveJPEG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create thumbnail %s: %w", path, err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return fmt.Errorf("encode jpeg: %w", err)
	}
	return nil
}

// isImageExt returns true for common image file extensions.
func isImageExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	}
	return false
}
