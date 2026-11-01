// Package markdown parses the small CommonMark subset used by the gothic
// documentation into a plain data model that gothic views can render.
package markdown

import (
	"regexp"
	"strconv"
	"strings"
)

// Block kinds.
const (
	KindHeading   = "heading"
	KindParagraph = "paragraph"
	KindCode      = "code"
	KindList      = "list"
	KindTable     = "table"
	KindQuote     = "quote"
	KindRule      = "rule"
	KindImage     = "image"
)

// Document is parsed markdown.
type Document struct {
	Blocks []Block
}

// Block is one top-level markdown block. The zero fields that do not apply to
// Kind are left empty so the model stays a single type the views can loop over.
type Block struct {
	Kind    string     `json:"Kind"`
	Level   int        `json:"Level,omitempty"`
	ID      string     `json:"ID,omitempty"`
	Spans   []Span     `json:"Spans,omitempty"`
	Code    string     `json:"Code"`
	Lang    string     `json:"Lang"`
	Ordered bool       `json:"Ordered,omitempty"`
	Items   []Item     `json:"Items,omitempty"`
	Header  [][]Span   `json:"Header,omitempty"`
	Rows    [][][]Span `json:"Rows,omitempty"`
	Src     string     `json:"Src,omitempty"`
	Alt     string     `json:"Alt,omitempty"`
}

// Item is one list item.
type Item struct {
	Spans []Span `json:"Spans,omitempty"`
}

// Span is inline content. Flags combine, so a link label can also be code or
// strong.
type Span struct {
	Text   string `json:"Text,omitempty"`
	Code   bool   `json:"Code,omitempty"`
	Strong bool   `json:"Strong,omitempty"`
	Em     bool   `json:"Em,omitempty"`
	Href   string `json:"Href,omitempty"`
	Image  bool   `json:"Image,omitempty"`
	Alt    string `json:"Alt,omitempty"`
}

// Plain returns the text of spans with formatting removed.
func Plain(spans []Span) string {
	var b strings.Builder
	for _, span := range spans {
		if span.Image {
			b.WriteString(span.Alt)
			continue
		}
		b.WriteString(span.Text)
	}
	return b.String()
}

var (
	headingLine = regexp.MustCompile(`^(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$`)
	fenceLine   = regexp.MustCompile("^(`{3,}|~{3,})[ \t]*([^ \t`]*)")
	listLine    = regexp.MustCompile(`^([ \t]*)([-*+]|[0-9]+\.)[ \t]+(.*)$`)
	ruleLine    = regexp.MustCompile(`^[ \t]*((\*[ \t]*){3,}|(-[ \t]*){3,}|(_[ \t]*){3,})$`)
	delimiter   = regexp.MustCompile(`^[ \t]*\|?[ \t]*:?-{3,}:?[ \t]*(\|[ \t]*:?-{3,}:?[ \t]*)*\|?[ \t]*$`)
	htmlImage   = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	htmlAttr    = regexp.MustCompile(`(?is)([a-z-]+)[ \t]*=[ \t]*"([^"]*)"`)
)

// Parse converts markdown source into a Document.
func Parse(src string) Document {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	lines := strings.Split(src, "\n")

	var doc Document
	slugs := map[string]int{}

	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			i++
			continue
		}

		if m := headingLine.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			spans := ParseInline(m[2])
			id := slug(Plain(spans))
			if id == "" {
				id = "section"
			}
			slugs[id]++
			if n := slugs[id]; n > 1 {
				id += "-" + strconv.Itoa(n)
			}
			doc.Blocks = append(doc.Blocks, Block{
				Kind:  KindHeading,
				Level: level,
				ID:    id,
				Spans: spans,
			})
			i++
			continue
		}

		if m := fenceLine.FindStringSubmatch(line); m != nil {
			marker := m[1]
			lang := strings.ToLower(m[2])
			var code []string
			i++
			for ; i < len(lines); i++ {
				if isFenceClose(lines[i], marker) {
					i++
					break
				}
				code = append(code, lines[i])
			}
			doc.Blocks = append(doc.Blocks, Block{
				Kind: KindCode,
				Code: strings.Join(code, "\n"),
				Lang: lang,
			})
			continue
		}

		if ruleLine.MatchString(line) {
			doc.Blocks = append(doc.Blocks, Block{Kind: KindRule})
			i++
			continue
		}

		if strings.HasPrefix(trimmed, "<") {
			if block, next, ok := parseRawImage(lines, i); ok {
				doc.Blocks = append(doc.Blocks, block)
				i = next
				continue
			}
			i = skipRawHTML(lines, i)
			continue
		}

		if isTableStart(lines, i) {
			block, next := parseTable(lines, i)
			doc.Blocks = append(doc.Blocks, block)
			i = next
			continue
		}

		if strings.HasPrefix(trimmed, ">") {
			spans, next := parseQuote(lines, i)
			doc.Blocks = append(doc.Blocks, Block{Kind: KindQuote, Spans: spans})
			i = next
			continue
		}

		if m := listLine.FindStringSubmatch(line); m != nil {
			block, next := parseList(lines, i)
			doc.Blocks = append(doc.Blocks, block)
			i = next
			continue
		}

		spans, next := parseParagraph(lines, i)
		doc.Blocks = append(doc.Blocks, Block{Kind: KindParagraph, Spans: spans})
		i = next
	}

	return doc
}

func isFenceClose(line, marker string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, marker) {
		return false
	}
	for _, r := range trimmed {
		if r != rune(marker[0]) {
			return false
		}
	}
	return true
}

func parseParagraph(lines []string, start int) ([]Span, int) {
	var parts []string
	i := start
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" || startsBlock(lines, i) {
			break
		}
		parts = append(parts, strings.TrimRight(line, " \t"))
	}
	return ParseInline(strings.Join(parts, "\n")), i
}

func startsBlock(lines []string, i int) bool {
	line := lines[i]
	trimmed := strings.TrimSpace(line)
	if headingLine.MatchString(line) || fenceLine.MatchString(line) || ruleLine.MatchString(line) {
		return true
	}
	if strings.HasPrefix(trimmed, ">") {
		return true
	}
	if listLine.MatchString(line) {
		return true
	}
	if strings.HasPrefix(trimmed, "<") {
		return true
	}
	return isTableStart(lines, i)
}

func parseQuote(lines []string, start int) ([]Span, int) {
	var parts []string
	i := start
	for ; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, ">") {
			break
		}
		content := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		if content == "" {
			continue
		}
		parts = append(parts, content)
	}
	return ParseInline(strings.Join(parts, "\n")), i
}

func parseList(lines []string, start int) (Block, int) {
	first := listLine.FindStringSubmatch(lines[start])
	ordered := first[2][0] >= '0' && first[2][0] <= '9'
	block := Block{Kind: KindList, Ordered: ordered}

	i := start
	for i < len(lines) {
		m := listLine.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}
		isOrdered := m[2][0] >= '0' && m[2][0] <= '9'
		if isOrdered != ordered {
			break
		}
		text := m[3]
		i++
		for i < len(lines) {
			next := lines[i]
			if strings.TrimSpace(next) == "" {
				break
			}
			if listLine.MatchString(next) {
				break
			}
			if !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
				break
			}
			text += " " + strings.TrimSpace(next)
			i++
		}
		block.Items = append(block.Items, Item{Spans: ParseInline(text)})
		if i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j >= len(lines) || listLine.FindStringSubmatch(lines[j]) == nil {
				i = j
				break
			}
			i = j
		}
	}
	return block, i
}

func isTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	if !strings.Contains(lines[i], "|") || !delimiter.MatchString(lines[i+1]) {
		return false
	}
	header := splitRow(lines[i])
	cols := splitRow(lines[i+1])
	return len(header) > 0 && len(header) == len(cols)
}

func parseTable(lines []string, start int) (Block, int) {
	block := Block{Kind: KindTable}
	for _, cell := range splitRow(lines[start]) {
		block.Header = append(block.Header, ParseInline(cell))
	}
	i := start + 2
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" || !strings.Contains(line, "|") {
			break
		}
		cells := splitRow(line)
		row := make([][]Span, len(block.Header))
		for c := range row {
			cell := ""
			if c < len(cells) {
				cell = cells[c]
			}
			row[c] = ParseInline(cell)
		}
		block.Rows = append(block.Rows, row)
	}
	return block, i
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var cells []string
	var b strings.Builder
	inCode := false
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			b.WriteRune(r)
			escaped = true
		case r == '`':
			inCode = !inCode
			b.WriteRune(r)
		case r == '|' && !inCode:
			cells = append(cells, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(b.String()))
	return cells
}

func parseRawImage(lines []string, start int) (Block, int, bool) {
	end := start
	var raw strings.Builder
	for ; end < len(lines); end++ {
		raw.WriteString(lines[end])
		raw.WriteString("\n")
		if strings.Contains(strings.ToLower(lines[end]), "</p>") {
			end++
			break
		}
		if end > start && strings.TrimSpace(lines[end]) == "" {
			break
		}
	}
	body := raw.String()
	if !strings.Contains(strings.ToLower(body), "align=\"center\"") && !strings.Contains(strings.ToLower(body), "align='center'") {
		return Block{}, start, false
	}
	img := htmlImage.FindString(body)
	if img == "" {
		return Block{}, start, false
	}
	attrs := map[string]string{}
	for _, m := range htmlAttr.FindAllStringSubmatch(img, -1) {
		attrs[strings.ToLower(m[1])] = m[2]
	}
	if attrs["src"] == "" {
		return Block{}, start, false
	}
	return Block{
		Kind: KindImage,
		Src:  attrs["src"],
		Alt:  attrs["alt"],
	}, end, true
}

func skipRawHTML(lines []string, start int) int {
	i := start
	for ; i < len(lines); i++ {
		line := strings.ToLower(lines[i])
		if strings.Contains(line, "</p>") || strings.Contains(line, "</div>") || strings.Contains(line, "</table>") {
			return i + 1
		}
		if i > start && strings.TrimSpace(lines[i]) == "" {
			return i
		}
	}
	return i
}

func slug(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	dash := false
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == ' ' || r == '-' || r == '_' || r == '/':
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
