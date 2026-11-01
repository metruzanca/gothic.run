package markdown

import (
	"testing"
)

func TestParseBlocks(t *testing.T) {
	src := `# Hello ` + "`world`" + `

Intro with **bold**, *em*, a [link](https://example.com), and ` + "`code`" + `.

## Steps

1. First
2. Second

- Alpha
- Beta

> A note with **weight**.

| Name | Value |
|---|---|
| ` + "`ui`" + ` | [path](docs/guide/README.md) |

` + "```go" + `
fmt.Println("hi")
` + "```" + `

---
`
	doc := Parse(src)
	if len(doc.Blocks) != 9 {
		t.Fatalf("got %d blocks, want 9: %#v", len(doc.Blocks), doc.Blocks)
	}

	h := doc.Blocks[0]
	if h.Kind != KindHeading || h.Level != 1 || h.ID != "hello-world" {
		t.Errorf("heading = %#v", h)
	}
	if Plain(h.Spans) != "Hello world" || !h.Spans[1].Code {
		t.Errorf("heading spans = %#v", h.Spans)
	}

	p := doc.Blocks[1]
	if p.Kind != KindParagraph || len(p.Spans) < 6 {
		t.Errorf("paragraph = %#v", p)
	}
	if p.Spans[1].Text != "bold" || !p.Spans[1].Strong {
		t.Errorf("bold span = %#v", p.Spans[1])
	}

	if doc.Blocks[2].Level != 2 || doc.Blocks[2].ID != "steps" {
		t.Errorf("h2 = %#v", doc.Blocks[2])
	}

	ordered := doc.Blocks[3]
	if ordered.Kind != KindList || !ordered.Ordered || len(ordered.Items) != 2 {
		t.Errorf("ordered list = %#v", ordered)
	}

	ul := doc.Blocks[4]
	if ul.Kind != KindList || ul.Ordered || len(ul.Items) != 2 {
		t.Errorf("unordered list = %#v", ul)
	}

	quote := doc.Blocks[5]
	if quote.Kind != KindQuote || Plain(quote.Spans) != "A note with weight." {
		t.Errorf("quote = %#v", quote)
	}

	table := doc.Blocks[6]
	if table.Kind != KindTable || len(table.Header) != 2 || len(table.Rows) != 1 {
		t.Fatalf("table = %#v", table)
	}
	if Plain(table.Header[0]) != "Name" || Plain(table.Rows[0][0]) != "ui" {
		t.Errorf("table cells = %#v", table)
	}
	if table.Rows[0][1][0].Href != "docs/guide/README.md" {
		t.Errorf("table link = %#v", table.Rows[0][1])
	}

	code := doc.Blocks[7]
	if code.Kind != KindCode || code.Lang != "go" || code.Code != `fmt.Println("hi")` {
		t.Errorf("code = %#v", code)
	}

	if doc.Blocks[8].Kind != KindRule {
		t.Errorf("rule = %#v", doc.Blocks[8])
	}
}

func TestParseStrongCodeAndLinkCode(t *testing.T) {
	spans := ParseInline("**`net/http` at the core.** See [`session`](x.md).")
	if len(spans) < 2 {
		t.Fatalf("spans = %#v", spans)
	}
	if !spans[0].Strong || !spans[0].Code || spans[0].Text != "net/http" {
		t.Errorf("strong code = %#v", spans[0])
	}
	var link Span
	for _, span := range spans {
		if span.Href == "x.md" {
			link = span
		}
	}
	if !link.Code || link.Text != "session" {
		t.Errorf("link code = %#v", link)
	}
}

func TestParsePlainCodeSamples(t *testing.T) {
	src := "```templ\n<p>{ model.Status }</p>\n<a href=\"/about\" route>About</a>\n```\n"
	doc := Parse(src)
	if len(doc.Blocks) != 1 || doc.Blocks[0].Code != "<p>{ model.Status }</p>\n<a href=\"/about\" route>About</a>" {
		t.Fatalf("code block = %#v", doc.Blocks)
	}
}

func TestParseRawImage(t *testing.T) {
	src := `<p align="center">
  <img src="assets/gothic-wordmark.svg" alt="Gothic" width="260">
</p>

Gothic is a framework.
`
	doc := Parse(src)
	if len(doc.Blocks) != 2 {
		t.Fatalf("blocks = %#v", doc.Blocks)
	}
	img := doc.Blocks[0]
	if img.Kind != KindImage || img.Src != "assets/gothic-wordmark.svg" || img.Alt != "Gothic" {
		t.Errorf("image = %#v", img)
	}
}

func TestParseDuplicateHeadings(t *testing.T) {
	doc := Parse("## Same\n\n## Same\n")
	if doc.Blocks[0].ID != "same" || doc.Blocks[1].ID != "same-2" {
		t.Errorf("ids = %q, %q", doc.Blocks[0].ID, doc.Blocks[1].ID)
	}
}

func TestParseAutolink(t *testing.T) {
	spans := ParseInline("Open <http://localhost:8080> now.")
	var found bool
	for _, span := range spans {
		if span.Href == "http://localhost:8080" && span.Text == "http://localhost:8080" {
			found = true
		}
	}
	if !found {
		t.Errorf("spans = %#v", spans)
	}
}

func TestParseParagraphStopsAtBlocks(t *testing.T) {
	doc := Parse("One line\nsecond line\n\n## Next\n")
	if len(doc.Blocks) != 2 || Plain(doc.Blocks[0].Spans) != "One line\nsecond line" {
		t.Fatalf("blocks = %#v", doc.Blocks)
	}
}
