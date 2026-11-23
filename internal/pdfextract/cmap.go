package pdfextract

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"unicode/utf16"
)

// CMap maps raw PDF character codes (1 or 2+ bytes) to Unicode strings
// according to a font's /ToUnicode CMap stream.
type CMap struct {
	codespace []codeRange
	mappings  map[uint32]map[int]string // byteLen -> code -> unicode string
	minBytes  int
	maxBytes  int
}

type codeRange struct {
	start   uint32
	end     uint32
	byteLen int
}

// ParseCMap parses a PostScript /ToUnicode CMap stream into a CMap lookup table.
func ParseCMap(data []byte) *CMap {
	cm := &CMap{
		mappings: make(map[uint32]map[int]string),
	}

	tokens := tokenizeCMap(data)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok {
		case "begincodespacerange":
			i++
			for i < len(tokens) && tokens[i] != "endcodespacerange" {
				if i+1 >= len(tokens) {
					break
				}
				sHex := tokens[i]
				eHex := tokens[i+1]
				i += 2
				if isHexToken(sHex) && isHexToken(eHex) {
					sVal, sLen := parseHexCode(sHex)
					eVal, _ := parseHexCode(eHex)
					if sLen > 0 {
						cm.codespace = append(cm.codespace, codeRange{
							start:   sVal,
							end:     eVal,
							byteLen: sLen,
						})
						if cm.minBytes == 0 || sLen < cm.minBytes {
							cm.minBytes = sLen
						}
						if sLen > cm.maxBytes {
							cm.maxBytes = sLen
						}
					}
				}
			}

		case "beginbfchar":
			i++
			for i < len(tokens) && tokens[i] != "endbfchar" {
				if i+1 >= len(tokens) {
					break
				}
				srcTok := tokens[i]
				dstTok := tokens[i+1]
				i += 2
				if isHexToken(srcTok) && isHexToken(dstTok) {
					code, bLen := parseHexCode(srcTok)
					uni := decodeUTF16Hex(dstTok)
					cm.setMapping(bLen, code, uni)
				}
			}

		case "beginbfrange":
			i++
			for i < len(tokens) && tokens[i] != "endbfrange" {
				if i+2 >= len(tokens) {
					break
				}
				startTok := tokens[i]
				endTok := tokens[i+1]
				i += 2
				if !isHexToken(startTok) || !isHexToken(endTok) {
					i++
					continue
				}
				startCode, bLen := parseHexCode(startTok)
				endCode, _ := parseHexCode(endTok)

				if i < len(tokens) && tokens[i] == "[" {
					i++ // consume '['
					cur := startCode
					for i < len(tokens) && tokens[i] != "]" {
						if isHexToken(tokens[i]) && cur <= endCode {
							cm.setMapping(bLen, cur, decodeUTF16Hex(tokens[i]))
							cur++
						}
						i++
					}
					if i < len(tokens) && tokens[i] == "]" {
						i++ // consume ']'
					}
				} else if i < len(tokens) {
					dstTok := tokens[i]
					i++
					if isHexToken(dstTok) {
						baseBytes := decodeHexBytes(dstTok)
						if len(baseBytes) > 0 {
							for code := startCode; code <= endCode; code++ {
								offset := code - startCode
								buf := make([]byte, len(baseBytes))
								copy(buf, baseBytes)
								addOffsetToBigEndian(buf, offset)
								cm.setMapping(bLen, code, decodeUTF16Bytes(buf))
							}
						}
					}
				}
			}
		}
	}

	if cm.minBytes == 0 {
		cm.minBytes = 1
	}
	if cm.maxBytes == 0 {
		cm.maxBytes = 2
	}
	return cm
}

func (c *CMap) setMapping(byteLen int, code uint32, uni string) {
	if byteLen <= 0 {
		return
	}
	if c.minBytes == 0 || byteLen < c.minBytes {
		c.minBytes = byteLen
	}
	if byteLen > c.maxBytes {
		c.maxBytes = byteLen
	}
	byCode, ok := c.mappings[uint32(byteLen)]
	if !ok {
		byCode = make(map[int]string)
		c.mappings[uint32(byteLen)] = byCode
	}
	byCode[int(code)] = uni
}

// DecodeBytes decodes raw character bytes from a PDF string using this CMap,
// returning the decoded Unicode string and the raw character codes consumed.
func (c *CMap) DecodeBytes(raw []byte) (string, []uint32) {
	if c == nil || len(raw) == 0 {
		return "", nil
	}
	var out strings.Builder
	var codes []uint32

	for i := 0; i < len(raw); {
		matchedLen := 0
		var matchedCode uint32
		var matchedStr string

		// Prefer codespace range lengths first, from shortest to longest or matching codespace.
		if len(c.codespace) > 0 {
			for _, cr := range c.codespace {
				if i+cr.byteLen > len(raw) {
					continue
				}
				code := bytesToUint32(raw[i : i+cr.byteLen])
				if code >= cr.start && code <= cr.end {
					matchedLen = cr.byteLen
					matchedCode = code
					if m, ok := c.mappings[uint32(cr.byteLen)]; ok {
						if s, found := m[int(code)]; found {
							matchedStr = s
						}
					}
					break
				}
			}
		}

		// Fallback: probe mapping lengths directly (preferring 2-byte if maxBytes >= 2).
		if matchedLen == 0 {
			for bLen := c.maxBytes; bLen >= c.minBytes; bLen-- {
				if i+bLen > len(raw) {
					continue
				}
				code := bytesToUint32(raw[i : i+bLen])
				if m, ok := c.mappings[uint32(bLen)]; ok {
					if s, found := m[int(code)]; found {
						matchedLen = bLen
						matchedCode = code
						matchedStr = s
						break
					}
				}
			}
		}

		if matchedLen == 0 {
			step := c.minBytes
			if i+step > len(raw) {
				step = len(raw) - i
			}
			matchedLen = step
			matchedCode = bytesToUint32(raw[i : i+step])
		}

		codes = append(codes, matchedCode)
		if matchedStr != "" {
			out.WriteString(matchedStr)
		} else if matchedCode >= 0x20 && matchedCode <= 0x7E {
			out.WriteByte(byte(matchedCode))
		}
		i += matchedLen
	}

	return out.String(), codes
}

func tokenizeCMap(data []byte) []string {
	var tokens []string
	n := len(data)
	for i := 0; i < n; {
		b := data[i]
		if isPDFWhitespace(b) {
			i++
			continue
		}
		if b == '%' {
			for i < n && data[i] != '\n' && data[i] != '\r' {
				i++
			}
			continue
		}
		if b == '<' {
			if i+1 < n && data[i+1] == '<' {
				tokens = append(tokens, "<<")
				i += 2
				continue
			}
			end := bytes.IndexByte(data[i+1:], '>')
			if end < 0 {
				tokens = append(tokens, string(data[i:]))
				break
			}
			tokens = append(tokens, string(data[i:i+end+2]))
			i += end + 2
			continue
		}
		if b == '>' && i+1 < n && data[i+1] == '>' {
			tokens = append(tokens, ">>")
			i += 2
			continue
		}
		if b == '[' || b == ']' {
			tokens = append(tokens, string(b))
			i++
			continue
		}
		start := i
		for i < n && !isPDFWhitespace(data[i]) && data[i] != '<' && data[i] != '>' && data[i] != '[' && data[i] != ']' && data[i] != '%' {
			i++
		}
		tokens = append(tokens, string(data[start:i]))
	}
	return tokens
}

func isHexToken(tok string) bool {
	return len(tok) >= 2 && tok[0] == '<' && tok[len(tok)-1] == '>'
}

func decodeHexBytes(tok string) []byte {
	inner := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, tok[1:len(tok)-1])
	if len(inner)%2 == 1 {
		inner += "0"
	}
	out, err := hex.DecodeString(inner)
	if err != nil {
		return nil
	}
	return out
}

func parseHexCode(tok string) (uint32, int) {
	b := decodeHexBytes(tok)
	return bytesToUint32(b), len(b)
}

func bytesToUint32(b []byte) uint32 {
	var v uint32
	for _, x := range b {
		v = (v << 8) | uint32(x)
	}
	return v
}

func addOffsetToBigEndian(buf []byte, offset uint32) {
	carry := offset
	for i := len(buf) - 1; i >= 0 && carry > 0; i-- {
		sum := uint32(buf[i]) + (carry & 0xFF)
		buf[i] = byte(sum & 0xFF)
		carry = (carry >> 8) + (sum >> 8)
	}
}

func decodeUTF16Hex(tok string) string {
	return decodeUTF16Bytes(decodeHexBytes(tok))
}

func decodeUTF16Bytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) == 1 {
		return string(rune(b[0]))
	}
	if len(b)%2 == 1 {
		b = append([]byte{0}, b...)
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = binary.BigEndian.Uint16(b[i*2 : i*2+2])
	}
	return string(utf16.Decode(u16))
}

func isPDFWhitespace(b byte) bool {
	return b == 0x00 || b == 0x09 || b == 0x0A || b == 0x0C || b == 0x0D || b == 0x20
}
