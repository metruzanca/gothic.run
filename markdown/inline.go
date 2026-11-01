package markdown

import "strings"

// ParseInline parses inline markdown into spans.
func ParseInline(s string) []Span {
	var spans []Span
	var text strings.Builder

	flush := func() {
		if text.Len() == 0 {
			return
		}
		spans = append(spans, Span{Text: text.String()})
		text.Reset()
	}

	for i := 0; i < len(s); {
		switch c := s[i]; c {
		case '\\':
			if i+1 < len(s) && isPunct(s[i+1]) {
				text.WriteByte(s[i+1])
				i += 2
				continue
			}
			text.WriteByte(c)
			i++

		case '`':
			n := runLength(s, i, '`')
			closeAt := findRun(s, i+n, '`', n)
			if closeAt < 0 {
				text.WriteString(s[i : i+n])
				i += n
				continue
			}
			content := strings.ReplaceAll(s[i+n:closeAt], "\n", " ")
			if len(content) >= 2 && content[0] == ' ' && content[len(content)-1] == ' ' && strings.TrimSpace(content) != "" {
				content = content[1 : len(content)-1]
			}
			flush()
			spans = append(spans, Span{Text: content, Code: true})
			i = closeAt + n

		case '*':
			if strings.HasPrefix(s[i:], "**") {
				if end := strings.Index(s[i+2:], "**"); end >= 0 && end > 0 {
					inner := ParseInline(s[i+2 : i+2+end])
					flush()
					for _, span := range inner {
						span.Strong = true
						spans = append(spans, span)
					}
					i = i + 2 + end + 2
					continue
				}
				text.WriteString("**")
				i += 2
				continue
			}
			if end := closingEm(s, i+1); end >= 0 {
				inner := ParseInline(s[i+1 : end])
				flush()
				for _, span := range inner {
					span.Em = true
					spans = append(spans, span)
				}
				i = end + 1
				continue
			}
			text.WriteByte(c)
			i++

		case '!':
			if i+1 < len(s) && s[i+1] == '[' {
				if alt, href, next, ok := parseLink(s, i+1); ok {
					flush()
					spans = append(spans, Span{Image: true, Alt: Plain(ParseInline(alt)), Href: href})
					i = next
					continue
				}
			}
			text.WriteByte(c)
			i++

		case '[':
			if label, href, next, ok := parseLink(s, i); ok {
				inner := ParseInline(label)
				flush()
				if len(inner) == 0 {
					inner = []Span{{Text: href}}
				}
				for _, span := range inner {
					span.Href = href
					spans = append(spans, span)
				}
				i = next
				continue
			}
			text.WriteByte(c)
			i++

		case '<':
			if end := strings.IndexByte(s[i:], '>'); end > 0 {
				target := s[i+1 : i+end]
				if isAutolink(target) {
					flush()
					spans = append(spans, Span{Text: target, Href: target})
					i += end + 1
					continue
				}
			}
			text.WriteByte(c)
			i++

		default:
			text.WriteByte(c)
			i++
		}
	}

	flush()
	return spans
}

func isPunct(c byte) bool {
	return strings.ContainsRune(`\`+"`*{}[]()#+-.!_<>|~", rune(c))
}

func runLength(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

func findRun(s string, from int, c byte, n int) int {
	for i := from; i < len(s); {
		if s[i] != c {
			i++
			continue
		}
		if runLength(s, i, c) == n {
			return i
		}
		i += runLength(s, i, c)
	}
	return -1
}

func closingEm(s string, from int) int {
	if from >= len(s) || s[from] == ' ' || s[from] == '*' {
		return -1
	}
	for i := from; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] != '*' {
			continue
		}
		if i == from || s[i-1] == ' ' {
			continue
		}
		return i
	}
	return -1
}

func parseLink(s string, open int) (label, href string, next int, ok bool) {
	if open >= len(s) || s[open] != '[' {
		return "", "", 0, false
	}
	closeLabel := -1
	for i := open + 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == ']' {
			closeLabel = i
			break
		}
	}
	if closeLabel < 0 || closeLabel+1 >= len(s) || s[closeLabel+1] != '(' {
		return "", "", 0, false
	}
	closeURL := -1
	depth := 0
	for i := closeLabel + 2; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			if depth == 0 {
				closeURL = i
				i = len(s)
				continue
			}
			depth--
		}
		if closeURL >= 0 {
			break
		}
	}
	if closeURL < 0 {
		return "", "", 0, false
	}
	label = s[open+1 : closeLabel]
	href = strings.TrimSpace(s[closeLabel+2 : closeURL])
	if strings.HasPrefix(href, "<") && strings.HasSuffix(href, ">") {
		href = href[1 : len(href)-1]
	}
	if idx := strings.Index(href, ` "`); idx >= 0 {
		href = href[:idx]
	}
	if idx := strings.Index(href, ` '`); idx >= 0 {
		href = href[:idx]
	}
	return label, strings.TrimSpace(href), closeURL + 1, true
}

func isAutolink(s string) bool {
	for _, prefix := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(strings.ToLower(s), prefix) && !strings.ContainsAny(s, " <>") {
			return true
		}
	}
	return false
}
