package pdfextract

import (
	"bytes"
	"strconv"
)

type tokenKind int

const (
	tokNumber tokenKind = iota
	tokString
	tokHexString
	tokName
	tokArray
	tokDict
	tokOperator
)

type token struct {
	kind tokenKind
	num  float64
	raw  []byte
	str  string
	arr  []token
}

// tokenizeContentStream lexes a PDF page or Form XObject content stream into tokens.
func tokenizeContentStream(data []byte) []token {
	s := &streamLexer{data: data}
	var out []token
	for {
		tok, ok := s.nextToken()
		if !ok {
			break
		}
		out = append(out, tok)
	}
	return out
}

type streamLexer struct {
	data []byte
	pos  int
}

func (l *streamLexer) skipSpaceAndComments() {
	n := len(l.data)
	for l.pos < n {
		b := l.data[l.pos]
		if isPDFWhitespace(b) {
			l.pos++
			continue
		}
		if b == '%' {
			for l.pos < n && l.data[l.pos] != '\n' && l.data[l.pos] != '\r' {
				l.pos++
			}
			continue
		}
		break
	}
}

func (l *streamLexer) nextToken() (token, bool) {
	l.skipSpaceAndComments()
	if l.pos >= len(l.data) {
		return token{}, false
	}

	b := l.data[l.pos]
	switch b {
	case '(':
		return token{kind: tokString, raw: l.readLiteralString()}, true
	case '<':
		if l.pos+1 < len(l.data) && l.data[l.pos+1] == '<' {
			l.readDict()
			return token{kind: tokDict}, true
		}
		return token{kind: tokHexString, raw: l.readHexString()}, true
	case '[':
		return token{kind: tokArray, arr: l.readArray()}, true
	case ']':
		l.pos++
		return token{kind: tokOperator, str: "]"}, true
	case '>':
		if l.pos+1 < len(l.data) && l.data[l.pos+1] == '>' {
			l.pos += 2
			return token{kind: tokOperator, str: ">>"}, true
		}
		l.pos++
		return token{kind: tokOperator, str: ">"}, true
	case '/':
		return token{kind: tokName, str: l.readName()}, true
	default:
		word := l.readWord()
		if word == "" {
			l.pos++
			return l.nextToken()
		}
		if word == "BI" {
			l.skipInlineImage()
			return token{kind: tokOperator, str: "BI"}, true
		}
		if isNumericWord(word) {
			if f, err := strconv.ParseFloat(word, 64); err == nil {
				return token{kind: tokNumber, num: f, str: word}, true
			}
		}
		return token{kind: tokOperator, str: word}, true
	}
}

func (l *streamLexer) readLiteralString() []byte {
	l.pos++ // skip '('
	var buf bytes.Buffer
	depth := 1
	n := len(l.data)
	for l.pos < n && depth > 0 {
		ch := l.data[l.pos]
		l.pos++
		switch ch {
		case '(':
			depth++
			buf.WriteByte(ch)
		case ')':
			depth--
			if depth > 0 {
				buf.WriteByte(ch)
			}
		case '\\':
			if l.pos >= n {
				break
			}
			esc := l.data[l.pos]
			l.pos++
			switch esc {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case '(', ')', '\\':
				buf.WriteByte(esc)
			case '\r':
				if l.pos < n && l.data[l.pos] == '\n' {
					l.pos++
				}
			case '\n':
				// line continuation
			default:
				if esc >= '0' && esc <= '7' {
					val := int(esc - '0')
					for k := 0; k < 2 && l.pos < n; k++ {
						d := l.data[l.pos]
						if d < '0' || d > '7' {
							break
						}
						val = val*8 + int(d-'0')
						l.pos++
					}
					buf.WriteByte(byte(val))
				} else {
					buf.WriteByte(esc)
				}
			}
		default:
			buf.WriteByte(ch)
		}
	}
	return buf.Bytes()
}

func (l *streamLexer) readHexString() []byte {
	l.pos++ // skip '<'
	var hexDigits []byte
	n := len(l.data)
	for l.pos < n {
		ch := l.data[l.pos]
		l.pos++
		if ch == '>' {
			break
		}
		if isPDFWhitespace(ch) {
			continue
		}
		hexDigits = append(hexDigits, ch)
	}
	if len(hexDigits)%2 == 1 {
		hexDigits = append(hexDigits, '0')
	}
	out := make([]byte, len(hexDigits)/2)
	for i := 0; i < len(out); i++ {
		hi := hexVal(hexDigits[i*2])
		lo := hexVal(hexDigits[i*2+1])
		out[i] = (hi << 4) | lo
	}
	return out
}

func hexVal(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	default:
		return 0
	}
}

func (l *streamLexer) readName() string {
	l.pos++ // skip '/'
	var buf bytes.Buffer
	n := len(l.data)
	for l.pos < n {
		ch := l.data[l.pos]
		if isPDFWhitespace(ch) || isDelimiter(ch) {
			break
		}
		l.pos++
		if ch == '#' && l.pos+1 < n {
			v := (hexVal(l.data[l.pos]) << 4) | hexVal(l.data[l.pos+1])
			buf.WriteByte(v)
			l.pos += 2
			continue
		}
		buf.WriteByte(ch)
	}
	return buf.String()
}

func (l *streamLexer) readArray() []token {
	l.pos++ // skip '['
	var arr []token
	for l.pos < len(l.data) {
		l.skipSpaceAndComments()
		if l.pos >= len(l.data) {
			break
		}
		if l.data[l.pos] == ']' {
			l.pos++
			break
		}
		tok, ok := l.nextToken()
		if !ok {
			break
		}
		arr = append(arr, tok)
	}
	return arr
}

func (l *streamLexer) readDict() {
	l.pos += 2 // skip '<<'
	depth := 1
	n := len(l.data)
	for l.pos < n && depth > 0 {
		ch := l.data[l.pos]
		if ch == '(' {
			_ = l.readLiteralString()
			continue
		}
		if ch == '<' && l.pos+1 < n && l.data[l.pos+1] == '<' {
			depth++
			l.pos += 2
			continue
		}
		if ch == '>' && l.pos+1 < n && l.data[l.pos+1] == '>' {
			depth--
			l.pos += 2
			continue
		}
		l.pos++
	}
}

func (l *streamLexer) readWord() string {
	start := l.pos
	n := len(l.data)
	for l.pos < n {
		ch := l.data[l.pos]
		if isPDFWhitespace(ch) || isDelimiter(ch) {
			break
		}
		l.pos++
	}
	return string(l.data[start:l.pos])
}

func (l *streamLexer) skipInlineImage() {
	n := len(l.data)
	for l.pos < n {
		if l.pos+2 <= n && l.data[l.pos] == 'I' && l.data[l.pos+1] == 'D' {
			l.pos += 2
			if l.pos < n && isPDFWhitespace(l.data[l.pos]) {
				l.pos++
			}
			break
		}
		l.pos++
	}
	for l.pos < n {
		if l.pos+2 <= n && l.data[l.pos] == 'E' && l.data[l.pos+1] == 'I' {
			prevOK := l.pos == 0 || isPDFWhitespace(l.data[l.pos-1])
			nextOK := l.pos+2 == n || isPDFWhitespace(l.data[l.pos+2])
			if prevOK && nextOK {
				l.pos += 2
				break
			}
		}
		l.pos++
	}
}

func isDelimiter(b byte) bool {
	switch b {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}

func isNumericWord(s string) bool {
	if s == "" {
		return false
	}
	hasDigit := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
			continue
		}
		if (c == '+' || c == '-') && i == 0 {
			continue
		}
		if c == '.' {
			continue
		}
		return false
	}
	return hasDigit
}
