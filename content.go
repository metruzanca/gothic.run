package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"unicode/utf8"

	"github.com/metruzanca/gothic.run/markdown"
)

const repoURL = "https://github.com/metruzanca/gothic"

// Source is one markdown file exposed as a page.
type Source struct {
	Path        string
	Route       string
	Title       string
	Nav         string
	Description string
}

var sources = []Source{
	{Path: "README.md", Route: "/", Title: "Gothic"},
	{Path: "docs/guide/README.md", Route: "/guide", Title: "Guide", Nav: "Guide"},
	{Path: "docs/guide/01-quickstart.md", Route: "/guide/quickstart", Title: "Quickstart", Nav: "Guide"},
	{Path: "docs/guide/02-state.md", Route: "/guide/state", Title: "State and commands", Nav: "Guide"},
	{Path: "docs/guide/03-components.md", Route: "/guide/components", Title: "Components", Nav: "Guide"},
	{Path: "docs/guide/04-routing.md", Route: "/guide/routing", Title: "Routing and documents", Nav: "Guide"},
	{Path: "docs/guide/05-browser.md", Route: "/guide/browser", Title: "Browser interop", Nav: "Guide"},
	{Path: "docs/guide/06-long-lists.md", Route: "/guide/long-lists", Title: "Long lists", Nav: "Guide"},
	{Path: "docs/guide/07-architecture.md", Route: "/guide/architecture", Title: "Architecture", Nav: "Guide"},
	{Path: "docs/guide/08-server.md", Route: "/guide/server", Title: "The server", Nav: "Guide"},
	{Path: "docs/SYNTAX.md", Route: "/syntax", Title: "Syntax", Nav: "Reference"},
	{Path: "docs/MODULES.md", Route: "/modules", Title: "Modules", Nav: "Reference"},
	{Path: "docs/PITCH.md", Route: "/pitch", Title: "Pitch", Nav: "Reference"},
	{Path: "docs/PLAN.md", Route: "/roadmap", Title: "Roadmap", Nav: "Reference"},
	{Path: "docs/RELEASING.md", Route: "/releasing", Title: "Releasing", Nav: "Reference"},
}

// NavGroup is a sidebar section.
type NavGroup struct {
	Title string
	Items []NavItem
}

// NavItem is a sidebar link.
type NavItem struct {
	Label  string
	Href   string
	Active bool
}

// TOCItem is one entry in the page table of contents.
type TOCItem struct {
	Level int
	Text  string
	Href  string
}

// NavLink is a previous or next page link.
type NavLink struct {
	Label string
	Href  string
}

// Page is one loaded document.
type Page struct {
	Source      Source
	Doc         markdown.Document
	Code        map[int]string
	Description string
	TOC         []TOCItem
	EditURL     string
	Prev        NavLink
	Next        NavLink
	HasPrev     bool
	HasNext     bool
}

// Site is every page plus navigation state.
type Site struct {
	Ref      string
	Pages    map[string]*Page
	Order    []string
	Nav      []NavGroup
	bySource map[string]string
}

func loadSite(dir string, list []Source) (*Site, error) {
	return newSite(os.DirFS(dir), list, gothicVersion())
}

func newSite(fsys fs.FS, list []Source, ref string) (*Site, error) {
	s := &Site{
		Ref:      ref,
		Pages:    map[string]*Page{},
		bySource: map[string]string{},
	}

	for _, source := range list {
		data, err := fs.ReadFile(fsys, source.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", source.Path, err)
		}
		doc := markdown.Parse(string(data))
		page := &Page{
			Source:      source,
			Doc:         doc,
			Description: source.Description,
			EditURL:     repoURL + "/blob/" + ref + "/" + source.Path,
		}
		if page.Description == "" {
			page.Description = describe(doc)
		}
		page.TOC = tableOfContents(doc)
		s.Pages[source.Route] = page
		s.Order = append(s.Order, source.Route)
		s.bySource[source.Path] = source.Route
	}

	for _, route := range s.Order {
		page := s.Pages[route]
		page.Doc = s.resolveDocument(page, page.Doc)
		page.Code = make(map[int]string)
		for i, block := range page.Doc.Blocks {
			if block.Kind == markdown.KindCode {
				page.Code[i] = highlight(block.Code, block.Lang)
			}
		}
	}

	for _, source := range list {
		if source.Nav == "" {
			continue
		}
		idx := -1
		for i := range s.Nav {
			if s.Nav[i].Title == source.Nav {
				idx = i
				break
			}
		}
		if idx < 0 {
			s.Nav = append(s.Nav, NavGroup{Title: source.Nav})
			idx = len(s.Nav) - 1
		}
		s.Nav[idx].Items = append(s.Nav[idx].Items, NavItem{Label: source.Title, Href: source.Route})
	}

	pages := s.Order
	if len(pages) > 0 && pages[0] == "/" {
		pages = pages[1:]
	}
	for i, route := range pages {
		page := s.Pages[route]
		if i > 0 {
			prev := s.Pages[pages[i-1]]
			page.Prev = NavLink{Label: prev.Source.Title, Href: prev.Source.Route}
			page.HasPrev = true
		}
		if i+1 < len(pages) {
			next := s.Pages[pages[i+1]]
			page.Next = NavLink{Label: next.Source.Title, Href: next.Source.Route}
			page.HasNext = true
		}
	}

	return s, nil
}

// CodeFor returns the highlighted code blocks for a route, keyed by block
// index. Unknown routes return nil.
func (s *Site) CodeFor(route string) map[int]string {
	if page, ok := s.Pages[cleanRoute(route)]; ok {
		return page.Code
	}
	return nil
}

// ModelAt builds the model for a route. The second result is false when the
// route is unknown; the returned model then renders the not found page.
func (s *Site) ModelAt(route string) (Model, bool) {
	route = cleanRoute(route)
	page, ok := s.Pages[route]
	if !ok {
		return s.notFound(), false
	}

	nav := make([]NavGroup, len(s.Nav))
	for i, group := range s.Nav {
		items := make([]NavItem, len(group.Items))
		copy(items, group.Items)
		for j := range items {
			items[j].Active = items[j].Href == route
		}
		nav[i] = NavGroup{Title: group.Title, Items: items}
	}

	title := page.Source.Title
	if route == "/" {
		title = "Gothic · a full-stack Go framework"
	} else {
		title += " · Gothic Docs"
	}

	return Model{
		Path:        route,
		DocTitle:    title,
		Description: page.Description,
		Wordmark:    wordmarkHref,
		Doc:         page.Doc,
		Nav:         nav,
		HomeActive:  route == "/",
		TOC:         page.TOC,
		HasPrev:     page.HasPrev,
		PrevLabel:   page.Prev.Label,
		PrevHref:    page.Prev.Href,
		HasNext:     page.HasNext,
		NextLabel:   page.Next.Label,
		NextHref:    page.Next.Href,
		HasEdit:     true,
		EditURL:     page.EditURL,
		Version:     s.Ref,
		VersionURL:  repoURL + "/tree/" + s.Ref,
		RepoURL:     repoURL,
	}, true
}

func (s *Site) notFound() Model {
	doc := markdown.Document{Blocks: []markdown.Block{
		{
			Kind:  markdown.KindHeading,
			Level: 1,
			ID:    "page-not-found",
			Spans: []markdown.Span{{Text: "Page not found"}},
		},
		{
			Kind: markdown.KindParagraph,
			Spans: []markdown.Span{
				{Text: "No page matches this path. "},
				{Text: "Start at the landing page", Href: "/"},
				{Text: ", or use the guide in the sidebar."},
			},
		},
	}}
	return Model{
		Path:        "/404",
		DocTitle:    "Page not found · Gothic Docs",
		Description: "No page matches this path.",
		Wordmark:    wordmarkHref,
		Doc:         doc,
		Nav:         s.Nav,
		Version:     s.Ref,
		VersionURL:  repoURL + "/tree/" + s.Ref,
		RepoURL:     repoURL,
		NotFound:    true,
	}
}

func cleanRoute(route string) string {
	if route == "" {
		return "/"
	}
	route = path.Clean("/" + route)
	if route != "/" {
		route = strings.TrimSuffix(route, "/")
	}
	return route
}

func describe(doc markdown.Document) string {
	for _, block := range doc.Blocks {
		if block.Kind != markdown.KindParagraph {
			continue
		}
		text := strings.TrimSpace(markdown.Plain(block.Spans))
		text = strings.Join(strings.Fields(text), " ")
		if text == "" {
			continue
		}
		return truncate(text, 160)
	}
	return ""
}

func truncate(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	cut := runes[:limit]
	if idx := strings.LastIndex(string(cut), " "); idx > 0 {
		cut = []rune(string(cut)[:idx])
	}
	return strings.TrimRight(string(cut), ".,;:") + "..."
}

func tableOfContents(doc markdown.Document) []TOCItem {
	var toc []TOCItem
	for _, block := range doc.Blocks {
		if block.Kind != markdown.KindHeading || block.Level < 2 || block.Level > 3 {
			continue
		}
		text := strings.TrimSpace(markdown.Plain(block.Spans))
		if text == "" {
			continue
		}
		toc = append(toc, TOCItem{
			Level: block.Level,
			Text:  text,
			Href:  "#" + block.ID,
		})
	}
	return toc
}

func (s *Site) resolveDocument(page *Page, doc markdown.Document) markdown.Document {
	for bi := range doc.Blocks {
		block := &doc.Blocks[bi]
		resolveSpans(s, page.Source.Path, block.Spans)
		for ii := range block.Items {
			resolveSpans(s, page.Source.Path, block.Items[ii].Spans)
		}
		for hi := range block.Header {
			resolveSpans(s, page.Source.Path, block.Header[hi])
		}
		for ri := range block.Rows {
			for ci := range block.Rows[ri] {
				resolveSpans(s, page.Source.Path, block.Rows[ri][ci])
			}
		}
		if block.Kind == markdown.KindImage {
			block.Src = s.resolveImage(page.Source.Path, block.Src)
		}
	}
	return doc
}

func resolveSpans(s *Site, from string, spans []markdown.Span) {
	for i := range spans {
		if spans[i].Href == "" {
			continue
		}
		if spans[i].Image {
			spans[i].Href = s.resolveImage(from, spans[i].Href)
			continue
		}
		spans[i].Href = s.resolveHref(from, spans[i].Href)
	}
}

func (s *Site) resolveHref(from, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return href
	}
	if hasScheme(href) || strings.HasPrefix(href, "//") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		return href
	}

	target, fragment := splitFragment(href)
	target = path.Clean(path.Join(path.Dir(from), target))
	if target == "." {
		return fragment
	}
	if route, ok := s.bySource[target]; ok {
		return route + fragment
	}
	return s.githubURL(target) + fragment
}

func (s *Site) resolveImage(from, src string) string {
	src = strings.TrimSpace(src)
	if src == "" || hasScheme(src) || strings.HasPrefix(src, "//") || strings.HasPrefix(src, "/") {
		return src
	}
	target := path.Clean(path.Join(path.Dir(from), src))
	switch target {
	case "assets/gothic-wordmark.svg", "assets/gothic-wordmark.png":
		return "/gothic-wordmark.svg"
	case "assets/gothic-mark.svg":
		return "/gothic-mark.svg"
	}
	return "https://raw.githubusercontent.com/metruzanca/gothic/" + s.Ref + "/" + target
}

func (s *Site) githubURL(target string) string {
	kind := "tree"
	if path.Ext(target) != "" {
		kind = "blob"
	}
	return repoURL + "/" + kind + "/" + s.Ref + "/" + target
}

func splitFragment(href string) (string, string) {
	if idx := strings.IndexByte(href, '#'); idx >= 0 {
		return href[:idx], href[idx:]
	}
	return href, ""
}

func hasScheme(href string) bool {
	for i := 0; i < len(href); i++ {
		c := href[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		case c == ':' && i > 0:
			return true
		default:
			return false
		}
	}
	return false
}

func gothicVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "main"
	}
	for _, dep := range info.Deps {
		if dep.Path != "github.com/metruzanca/gothic" {
			continue
		}
		if dep.Replace != nil || dep.Version == "" || dep.Version == "(devel)" {
			return "main"
		}
		return dep.Version
	}
	return "main"
}
