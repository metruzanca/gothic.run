package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/metruzanca/gothic"
)

var (
	gothicComments = regexp.MustCompile(`<!--.*?-->`)
	gothicAttrs    = regexp.MustCompile(` data-gothic-[a-z0-9-]+="[^"]*"`)
)

// clean strips the hydration markers and bookkeeping attributes gothic adds so
// tests can assert on the markup a reader sees.
func clean(html string) string {
	html = gothicComments.ReplaceAllString(html, "")
	return gothicAttrs.ReplaceAllString(html, "")
}

var testSources = []Source{
	{Path: "README.md", Route: "/", Title: "Gothic"},
	{Path: "docs/guide/README.md", Route: "/guide", Title: "Guide", Nav: "Guide"},
	{Path: "docs/guide/01-quickstart.md", Route: "/guide/quickstart", Title: "Quickstart", Nav: "Guide"},
	{Path: "docs/SYNTAX.md", Route: "/syntax", Title: "Syntax", Nav: "Reference"},
}

func testSite(t *testing.T) *Site {
	t.Helper()
	site, err := newSite(os.DirFS("testdata/content"), testSources, "v0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	return site
}

func testServer(t *testing.T, site *Site) *httptest.Server {
	t.Helper()
	s := gothic.NewServer(gothic.WithHealth(gothic.HealthPath))
	s.Asset("/style.css", styleCSS)
	s.Asset("/gothic-wordmark.svg", wordmarkSVG)
	s.Asset("/gothic-mark.svg", markSVG)
	s.Asset("/og.png", ogPNG)
	handle := pageHandler(site)
	s.Get("/", handle)
	s.Get("/*", handle)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return ts
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(body)
}

func TestLanding(t *testing.T) {
	site := testSite(t)
	ts := testServer(t, site)
	status, html := get(t, ts.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	for _, want := range []string{
		"<!doctype html>",
		"<title>Gothic · a full-stack Go framework</title>",
		`<meta name="description" content="Gothic is a batteries-included Go framework for building full-stack apps.`,
		`<meta property="og:image" content="` + ts.URL + `/og.png?v=`,
		`<link rel="icon" href="/gothic-mark.svg" type="image/svg+xml">`,
		`<link rel="stylesheet" href="/style.css?v=`,
		`class="hero"`,
		`src="/gothic-wordmark.svg?v=`,
		`id="goals"`,
		`Goals`,
		`href="/guide/quickstart"`,
		`https://github.com/metruzanca/gothic/tree/v0.4.0/examples/guestbook`,
		`https://github.com/metruzanca/gothic/tree/v0.4.0`,
		`data-theme-toggle`,
		`data-copy`,
		`prefers-color-scheme: dark`,
		`class="nav-link active"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("landing missing %q", want)
		}
	}
}

func TestGuidePage(t *testing.T) {
	site := testSite(t)
	ts := testServer(t, site)
	status, raw := get(t, ts.URL+"/guide/quickstart")
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if strings.Contains(raw, "data-gothic-state") || strings.Contains(raw, "/_gothic/modules/") {
		t.Errorf("page still ships the gothic hydration payload")
	}
	if strings.Contains(raw, "<!--g:") || strings.Contains(raw, "data-gothic-") {
		t.Errorf("page still contains gothic hydration markers")
	}
	if strings.Contains(raw, "data-icon") {
		t.Errorf("page still contains an icon placeholder")
	}
	for _, want := range []string{
		`aria-controls="sidebar"`,
		`class="nav-toggle"`,
		`<svg xmlns="http://www.w3.org/2000/svg"`,
		`class="nav-icon"`,
		`Docs for <a href="https://github.com/metruzanca/gothic/tree/v0.4.0">v0.4.0</a>`,
		`<span>Gothic v0.4.0</span>`,
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("page missing %q", want)
		}
	}
	html := clean(raw)
	for _, want := range []string{
		"<title>Quickstart · Gothic Docs</title>",
		`<h1 id="01-quickstart">`,
		`01 Quickstart`,
		`<h2 id="create-a-project">`,
		`Create a project`,
		`On this page`,
		`<table>`,
		`<blockquote>`,
		`<code>go get github.com/metruzanca/gothic</code>`,
		`class="pager-link pager-prev"`,
		`class="pager-link pager-next"`,
		`Edit this page on GitHub`,
		`https://github.com/metruzanca/gothic/blob/v0.4.0/docs/guide/01-quickstart.md`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("guide missing %q", want)
		}
	}
	if strings.Count(html, `class="nav-link active"`) != 1 {
		t.Errorf("expected exactly one active nav link")
	}
	if !strings.Contains(html, `<a href="/guide/quickstart" class="nav-link active">`) {
		t.Errorf("quickstart is not the active nav item")
	}
}

func TestNotFound(t *testing.T) {
	ts := testServer(t, testSite(t))
	status, raw := get(t, ts.URL+"/does-not-exist")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
	html := clean(raw)
	for _, want := range []string{
		"<title>Page not found · Gothic Docs</title>",
		`<h1 id="page-not-found">`,
		`Page not found`,
		`No page matches this path.`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("not found missing %q", want)
		}
	}
}

func TestHealth(t *testing.T) {
	ts := testServer(t, testSite(t))
	status, body := get(t, ts.URL+"/healthz")
	if status != http.StatusOK || !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("health = %d %s", status, body)
	}
}

func TestAssets(t *testing.T) {
	ts := testServer(t, testSite(t))
	for _, path := range []string{"/style.css", "/gothic-wordmark.svg", "/gothic-mark.svg", "/og.png"} {
		status, body := get(t, ts.URL+path)
		if status != http.StatusOK || body == "" {
			t.Errorf("GET %s = %d, %d bytes", path, status, len(body))
		}
	}
}

func TestLinkResolution(t *testing.T) {
	site := testSite(t)
	cases := []struct {
		from, href, want string
	}{
		{"README.md", "docs/guide/01-quickstart.md", "/guide/quickstart"},
		{"README.md", "docs/SYNTAX.md#views", "/syntax#views"},
		{"docs/guide/01-quickstart.md", "README.md", "/guide"},
		{"docs/guide/01-quickstart.md", "../SYNTAX.md", "/syntax"},
		{"README.md", "examples/guestbook", "https://github.com/metruzanca/gothic/tree/v0.4.0/examples/guestbook"},
		{"README.md", "CREDITS.md", "https://github.com/metruzanca/gothic/blob/v0.4.0/CREDITS.md"},
		{"README.md", "https://example.com/x", "https://example.com/x"},
		{"README.md", "#goals", "#goals"},
	}
	for _, c := range cases {
		if got := site.resolveHref(c.from, c.href); got != c.want {
			t.Errorf("resolveHref(%q, %q) = %q, want %q", c.from, c.href, got, c.want)
		}
	}
	if got := site.resolveImage("README.md", "assets/gothic-wordmark.svg"); got != "/gothic-wordmark.svg" {
		t.Errorf("wordmark image = %q", got)
	}
	if got := site.resolveImage("README.md", "assets/other.png"); got != "https://raw.githubusercontent.com/metruzanca/gothic/v0.4.0/assets/other.png" {
		t.Errorf("raw image = %q", got)
	}
}

func TestManifestRoutesAreUnique(t *testing.T) {
	routes := map[string]bool{}
	paths := map[string]bool{}
	for _, source := range sources {
		if routes[source.Route] {
			t.Errorf("duplicate route %q", source.Route)
		}
		if paths[source.Path] {
			t.Errorf("duplicate source %q", source.Path)
		}
		routes[source.Route] = true
		paths[source.Path] = true
	}
}

func TestRealContent(t *testing.T) {
	if _, err := os.Stat("content/README.md"); err != nil {
		t.Skip("content/ is not synced; run make docs")
	}
	site, err := loadSite("content", sources)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range site.Order {
		page := site.Pages[route]
		if len(page.Doc.Blocks) == 0 {
			t.Errorf("%s (%s) rendered no blocks", route, page.Source.Path)
		}
		for _, block := range page.Doc.Blocks {
			if block.Kind == "" {
				t.Errorf("%s has a block without a kind", route)
			}
		}
	}
}
