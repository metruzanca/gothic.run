//go:generate go run github.com/metruzanca/gothic/cmd/gothic generate .

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/kaugesaar/lucide-go"
	"github.com/metruzanca/gothic"
)

//go:embed style.css
var styleCSS []byte

//go:embed assets/gothic-wordmark.svg
var wordmarkSVG []byte

//go:embed assets/gothic-mark.svg
var markSVG []byte

//go:embed assets/og.png
var ogPNG []byte

// Asset URLs carry a content hash so a deploy is never served a stale
// stylesheet or social card from the Railway edge cache.
var (
	styleHref    = assetHref("/style.css", styleCSS)
	wordmarkHref = assetHref("/gothic-wordmark.svg", wordmarkSVG)
	ogImageHref  = assetHref("/og.png", ogPNG)
)

func assetHref(path string, data []byte) string {
	sum := sha256.Sum256(data)
	return path + "?v=" + hex.EncodeToString(sum[:6])
}

// headScript boots the stored theme before first paint and delegates the few
// pieces of client behavior the docs need: theme toggle, copy buttons, and the
// mobile navigation drawer.
const headScript = `<script>
(() => {
	const root = document.documentElement;
	try {
		const saved = localStorage.getItem('gothic-theme');
		if (saved === 'light' || saved === 'dark') root.dataset.theme = saved;
	} catch (error) {}

	const media = window.matchMedia('(prefers-color-scheme: dark)');
	const setTheme = (value) => {
		root.dataset.theme = value;
		try { localStorage.setItem('gothic-theme', value); } catch (error) {}
	};

	document.addEventListener('click', (event) => {
		if (!(event.target instanceof Element)) return;

		const themeButton = event.target.closest('[data-theme-toggle]');
		if (themeButton) {
			const current = root.dataset.theme || (media.matches ? 'dark' : 'light');
			setTheme(current === 'dark' ? 'light' : 'dark');
			return;
		}

		const copyButton = event.target.closest('[data-copy]');
		if (copyButton) {
			const block = copyButton.closest('.code');
			const code = block && block.querySelector('pre code');
			if (!code) return;
			const done = () => {
				copyButton.textContent = 'Copied';
				setTimeout(() => { copyButton.textContent = 'Copy'; }, 1500);
			};
			if (navigator.clipboard) {
				navigator.clipboard.writeText(code.textContent).then(done, () => {});
			}
			return;
		}

		const navButton = event.target.closest('[data-nav-toggle]');
		if (navButton) {
			const open = document.body.classList.toggle('nav-open');
			navButton.setAttribute('aria-expanded', open ? 'true' : 'false');
			return;
		}

		if (event.target.closest('.sidebar a')) {
			document.body.classList.remove('nav-open');
			const toggle = document.querySelector('[data-nav-toggle]');
			if (toggle) toggle.setAttribute('aria-expanded', 'false');
		}
	});

	document.addEventListener('keydown', (event) => {
		if (event.key !== 'Escape') return;
		document.body.classList.remove('nav-open');
		const toggle = document.querySelector('[data-nav-toggle]');
		if (toggle) toggle.setAttribute('aria-expanded', 'false');
	});
})();
</script>`

func main() {
	site, err := loadSite(docsDir(), sources)
	if err != nil {
		log.Fatalf("load docs: %v\n\nRun `make docs` or set DOCS_DIR to a gothic checkout ending in README.md and docs/.", err)
	}

	s := gothic.NewServer(gothic.WithHealth(gothic.HealthPath))
	s.Asset("/style.css", styleCSS)
	s.Asset("/gothic-wordmark.svg", wordmarkSVG)
	s.Asset("/gothic-mark.svg", markSVG)
	s.Asset("/og.png", ogPNG)

	handle := pageHandler(site)
	s.Get("/", handle)
	s.Get("/*", handle)

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	log.Printf("gothic docs %s listening on %s (markdown from %s)", site.Ref, addr, docsDir())
	log.Fatal(s.ListenAndServe(addr))
}

func docsDir() string {
	if dir := os.Getenv("DOCS_DIR"); dir != "" {
		return dir
	}
	return "content"
}

func pageHandler(site *Site) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		model, found := site.ModelAt(r.URL.Path)
		header := w.Header()
		header.Set("Content-Type", "text/html; charset=utf-8")
		header.Set("Cache-Control", "no-cache")
		if !found {
			w.WriteHeader(http.StatusNotFound)
		}
		doc := gothic.Document{
			Title:       model.DocTitle,
			Description: model.Description,
			SiteName:    "Gothic",
			Image:       ogImageHref,
			Icons:       []gothic.Icon{{Href: "/gothic-mark.svg", Type: "image/svg+xml"}},
			Styles:      []string{styleHref},
			ExtraHead:   headScript,
		}
		if err := gothic.WriteDocument(w, r, doc, staticHTML(App(model))); err != nil {
			log.Printf("render %s: %v", r.URL.Path, err)
		}
	}
}

// menuIcon is the lucide menu glyph used by the mobile navigation trigger.
var menuIcon = string(lucide.Menu(lucide.Options{Size: 20, Color: "currentColor", Class: "nav-icon"}))

var (
	stateScript     = regexp.MustCompile(`(?s)<script type="application/json" data-gothic-state="[^"]*">.*?</script>`)
	moduleScript    = regexp.MustCompile(`(?s)\n?<script type="module" src="[^"]*/_gothic/modules/[^"]+"></script>`)
	gothicMarker    = regexp.MustCompile(`(?s)<!--g:.*?-->`)
	gothicDataAttrs = regexp.MustCompile(` data-gothic-[a-z0-9-]+="[^"]*"`)
)

// staticHTML renders a gothic view and removes the hydration payload, module
// script, and bookkeeping markers. The docs site has no client state, so every
// page is fully server rendered and ships no application JavaScript.
func staticHTML(view gothic.Component) gothic.Component {
	return gothic.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		var buf bytes.Buffer
		if err := view.Render(ctx, &buf); err != nil {
			return err
		}
		html := buf.String()
		html = stateScript.ReplaceAllString(html, "")
		html = moduleScript.ReplaceAllString(html, "")
		html = gothicMarker.ReplaceAllString(html, "")
		html = gothicDataAttrs.ReplaceAllString(html, "")
		html = strings.ReplaceAll(html, `<span data-icon="menu"></span>`, menuIcon)
		_, err := io.WriteString(w, html)
		return err
	})
}
