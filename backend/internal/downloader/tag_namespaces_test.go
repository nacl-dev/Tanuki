package downloader

import (
	"slices"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

func TestQualifyTags(t *testing.T) {
	t.Parallel()

	got := qualifyTags("artist", []string{" John Doe ", "John Doe", "", "Jane"})
	want := []string{"artist:John Doe", "artist:Jane"}
	if !slices.Equal(got, want) {
		t.Fatalf("qualifyTags mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestBooruAllTagsNamespaced(t *testing.T) {
	t.Parallel()

	post := booruPost{
		General:    []string{"solo"},
		Artists:    []string{"creator_name"},
		Characters: []string{"heroine"},
		Copyrights: []string{"sample_series"},
		Meta:       []string{"highres"},
		Site:       "gelbooru",
		Rating:     "e",
	}

	got := post.AllTags()
	for _, want := range []string{"solo", "artist:creator_name", "character:heroine", "series:sample_series", "meta:highres", "site:gelbooru", "rating:explicit"} {
		if !slices.Contains(got, want) {
			t.Fatalf("expected tag %q in %#v", want, got)
		}
	}
}

func TestDanbooruAllTagsNamespaced(t *testing.T) {
	t.Parallel()

	post := danbooruPost{
		TagStringGeneral:   "solo smile",
		TagStringArtist:    "creator_name",
		TagStringCharacter: "heroine",
		TagStringCopyright: "sample_series",
		TagStringMeta:      "highres",
		Site:               "danbooru",
		Rating:             "q",
	}

	got := post.AllTags()
	for _, want := range []string{"solo", "smile", "artist:creator name", "character:heroine", "series:sample series", "meta:highres", "site:danbooru", "rating:questionable"} {
		if !slices.Contains(got, want) {
			t.Fatalf("expected tag %q in %#v", want, got)
		}
	}
}

func TestExtractYtDlpTagsNamespaced(t *testing.T) {
	t.Parallel()

	got := extractYtDlpTags(map[string]interface{}{
		"tags":       []interface{}{"raw-tag"},
		"artist":     "Lead Artist",
		"creators":   []interface{}{"Co Creator"},
		"series":     "Example Series",
		"categories": []interface{}{"Animation"},
		"language":   "en",
		"channel":    "Source Channel",
	})

	for _, want := range []string{
		"raw-tag",
		"artist:Lead Artist",
		"artist:Co Creator",
		"series:Example Series",
		"genre:Animation",
		"language:en",
		"uploader:Source Channel",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("expected tag %q in %#v", want, got)
		}
	}
}

func TestPornComicsTagsNamespaced(t *testing.T) {
	t.Parallel()

	comic := pornComicsComic{
		Authors: []pornComicsLabel{
			{Name: "Lead Artist"},
			{Name: "Lead Artist"},
		},
		Sections: []pornComicsLabel{
			{Name: "Premium"},
		},
		Characters: []pornComicsLabel{
			{Name: "Heroine"},
		},
		Categories: []pornComicsLabel{
			{Name: "Adventure"},
			{Name: "Adventure"},
		},
	}

	got := comic.Tags()
	for _, want := range []string{
		"artist:Lead Artist",
		"site:Premium",
		"character:Heroine",
		"genre:Adventure",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("expected tag %q in %#v", want, got)
		}
	}
	if len(got) != 4 {
		t.Fatalf("expected deduplicated tags, got %#v", got)
	}
}

func TestBuildRule34ArtTagsNamespaced(t *testing.T) {
	t.Parallel()

	comicTags := buildRule34ArtComicTags(
		" English ",
		[]string{"Jane Doe", "Jane Doe"},
		[]string{"Featured"},
	)
	if !slices.Equal(comicTags, []string{
		"language:English",
		"artist:Jane Doe",
		"site:Featured",
	}) {
		t.Fatalf("unexpected comic tags: %#v", comicTags)
	}

	videoTags := buildRule34ArtVideoTags(
		[]string{"monster", "monster"},
		[]string{"Animator"},
		[]string{"Video"},
	)
	if !slices.Equal(videoTags, []string{
		"genre:monster",
		"artist:Animator",
		"site:Video",
	}) {
		t.Fatalf("unexpected video tags: %#v", videoTags)
	}
}

func TestExtractDoujinsTagsNamespaced(t *testing.T) {
	t.Parallel()

	html := `
		<html>
			<head>
				<meta name="description" content="Example. Tags: Mind Break, Monster Girl and Sci-Fi." />
			</head>
			<body></body>
		</html>
	`

	doc, err := xhtml.Parse(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	got := extractDoujinsTags(doc)
	for _, want := range []string{
		"site:doujins.com",
		"genre:Mind Break",
		"genre:Monster Girl",
		"genre:Sci-Fi",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("expected tag %q in %#v", want, got)
		}
	}
}

func TestHentai0TagsNamespaced(t *testing.T) {
	t.Parallel()

	video := &hentai0Video{
		Tags: []string{"site:hentai0.com"},
	}
	for _, name := range []string{"Schoolgirl", "Schoolgirl", "Dubbed"} {
		video.Tags = append(video.Tags, qualifyTags("genre", []string{name})...)
	}
	video.Tags = compactStrings(video.Tags)

	for _, want := range []string{
		"site:hentai0.com",
		"genre:Schoolgirl",
		"genre:Dubbed",
	} {
		if !slices.Contains(video.Tags, want) {
			t.Fatalf("expected tag %q in %#v", want, video.Tags)
		}
	}
}
