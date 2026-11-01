package main

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// lexerAliases maps fence languages to the closest Chroma lexer.
var lexerAliases = map[string]string{
	"shell":   "bash",
	"sh":      "bash",
	"console": "bash",
}

var viewStart = regexp.MustCompile(`(?m)^view[ \t]+[A-Za-z_]`)

var highlightFormatter = chromahtml.New(
	chromahtml.WithClasses(true),
	chromahtml.PreventSurroundingPre(true),
)

// highlight renders one code block as Chroma token spans. Unknown or missing
// languages fall back to escaped plain text.
func highlight(code, lang string) string {
	name := strings.ToLower(strings.TrimSpace(lang))
	if name == "templ" || name == "gth" {
		return highlightTempl(code)
	}
	if alias, ok := lexerAliases[name]; ok {
		name = alias
	}
	if name == "" {
		return html.EscapeString(code)
	}
	return highlightChunk(code, name)
}

// highlightTempl highlights the Go prelude with the Go lexer and the views
// with the HTML lexer. Gothic is not a Go template, so Chroma has no lexer for
// the whole file.
func highlightTempl(code string) string {
	if match := viewStart.FindStringIndex(code); match != nil {
		return highlightTemplChunk(code[:match[0]], "go") + highlightTemplChunk(code[match[0]:], "html")
	}
	if strings.HasPrefix(strings.TrimSpace(code), "package ") {
		return highlightTemplChunk(code, "go")
	}
	return highlightTemplChunk(code, "html")
}

func highlightTemplChunk(code, name string) string {
	if strings.TrimSpace(code) == "" {
		return code
	}
	return highlightChunk(code, name)
}

func highlightChunk(code, name string) string {
	lexer := lexers.Get(name)
	if lexer == nil {
		return html.EscapeString(code)
	}
	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return html.EscapeString(code)
	}
	var buf bytes.Buffer
	if err := highlightFormatter.Format(&buf, styles.Get("github"), iterator); err != nil {
		return html.EscapeString(code)
	}
	return buf.String()
}

// chromaCSS builds the light and dark token styles. Dark rules are scoped so
// the site's explicit theme toggle and the system preference both work.
func chromaCSS() []byte {
	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.WithCSSComments(false),
	)
	var light, dark bytes.Buffer
	formatter.WriteCSS(&light, styles.Get("github"))
	formatter.WriteCSS(&dark, styles.Get("github-dark"))

	lightCSS := light.String()
	darkCSS := dark.String()

	var out strings.Builder
	out.WriteString("@media (prefers-color-scheme: light) {\n")
	out.WriteString(scopeCSS(lightCSS, `:root:not([data-theme="dark"])`))
	out.WriteString("\n}\n")
	out.WriteString(scopeCSS(lightCSS, `:root[data-theme="light"]`))
	out.WriteString("\n@media (prefers-color-scheme: dark) {\n")
	out.WriteString(scopeCSS(darkCSS, `:root:not([data-theme="light"])`))
	out.WriteString("\n}\n")
	out.WriteString(scopeCSS(darkCSS, `:root[data-theme="dark"]`))
	out.WriteString("\n")
	return []byte(out.String())
}

func scopeCSS(css, scope string) string {
	return strings.ReplaceAll(css, ".chroma", scope+" .chroma")
}
