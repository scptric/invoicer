package pdfextract

import (
	"fmt"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// Font holds the decoding tables and glyph metrics for one PDF font resource.
type Font struct {
	Name         string
	BaseFont     string
	Subtype      string
	Bold         bool
	ToUnicode    *CMap
	Encoding     singleByteEncoding
	DefaultWidth float64
	Widths       map[uint32]float64
}

// Decode converts raw string bytes from a Tj/TJ operand into Unicode text,
// total advance width in 1/1000 em units, decoded character count, and ASCII space count.
func (f *Font) Decode(raw []byte) (text string, totalWidth1000 float64, charCount int, spaceCount int) {
	if len(raw) == 0 {
		return "", 0, 0, 0
	}
	if f == nil {
		s := string(raw)
		for _, r := range s {
			charCount++
			if r == ' ' {
				spaceCount++
			}
			totalWidth1000 += 500
		}
		return s, totalWidth1000, charCount, spaceCount
	}

	if f.ToUnicode != nil {
		txt, codes := f.ToUnicode.DecodeBytes(raw)
		for _, code := range codes {
			totalWidth1000 += f.glyphWidth(code)
		}
		for _, r := range txt {
			charCount++
			if r == ' ' {
				spaceCount++
			}
		}
		return txt, totalWidth1000, charCount, spaceCount
	}

	if f.Subtype == "Type0" {
		// Identity-H / 2-byte fallback if ToUnicode was omitted
		var b strings.Builder
		for i := 0; i+1 < len(raw); i += 2 {
			code := uint32(raw[i])<<8 | uint32(raw[i+1])
			totalWidth1000 += f.glyphWidth(code)
			charCount++
			if code == ' ' {
				spaceCount++
			}
			if code >= 0x20 && code <= 0xFFFF {
				b.WriteRune(rune(code))
			}
		}
		return b.String(), totalWidth1000, charCount, spaceCount
	}

	txt := f.Encoding.Decode(raw)
	for _, b := range raw {
		totalWidth1000 += f.glyphWidth(uint32(b))
	}
	for _, r := range txt {
		charCount++
		if r == ' ' {
			spaceCount++
		}
	}
	return txt, totalWidth1000, charCount, spaceCount
}

func (f *Font) glyphWidth(code uint32) float64 {
	if w, ok := f.Widths[code]; ok {
		return w
	}
	if f.DefaultWidth > 0 {
		return f.DefaultWidth
	}
	return 500
}

// loadPageFonts resolves all /Font entries in a consolidated page /Resources dict.
func loadPageFonts(xRef *model.XRefTable, resources types.Dict, decodeLimit int64) map[string]*Font {
	fonts := make(map[string]*Font)
	if resources == nil {
		return fonts
	}
	fontObj, found := resources.Find("Font")
	if !found {
		return fonts
	}
	fontDict, err := xRef.DereferenceDict(fontObj)
	if err != nil || fontDict == nil {
		return fonts
	}
	for name, entry := range fontDict {
		fd, err := xRef.DereferenceDict(entry)
		if err != nil || fd == nil {
			continue
		}
		f := parseFontDict(xRef, name, fd, decodeLimit)
		fonts[name] = f
	}
	return fonts
}

func parseFontDict(xRef *model.XRefTable, name string, d types.Dict, decodeLimit int64) *Font {
	f := &Font{
		Name:         name,
		DefaultWidth: 500,
		Widths:       make(map[uint32]float64),
		Encoding:     initEncodingTable("WinAnsiEncoding"),
	}
	if st := d.Subtype(); st != nil {
		f.Subtype = *st
	}
	if bf := d.NameEntry("BaseFont"); bf != nil {
		f.BaseFont = *bf
		lower := strings.ToLower(f.BaseFont)
		if strings.Contains(lower, "bold") || strings.Contains(lower, "black") || strings.Contains(lower, "heavy") {
			f.Bold = true
		}
	}

	if tuObj, ok := d.Find("ToUnicode"); ok {
		if sd, _, err := xRef.DereferenceStreamDict(tuObj); err == nil && sd != nil {
			if err := sd.DecodeWithLimit(decodeLimit); err == nil {
				f.ToUnicode = ParseCMap(sd.Content)
			}
		}
	}

	if f.Subtype == "Type0" {
		f.DefaultWidth = 1000
		if dfObj, ok := d.Find("DescendantFonts"); ok {
			if arr, err := xRef.DereferenceArray(dfObj); err == nil && len(arr) > 0 {
				if descDict, err := xRef.DereferenceDict(arr[0]); err == nil && descDict != nil {
					if dw, ok := numericEntry(xRef, descDict, "DW"); ok {
						f.DefaultWidth = dw
					}
					if wObj, ok := descDict.Find("W"); ok {
						if wArr, err := xRef.DereferenceArray(wObj); err == nil {
							parseCIDWidths(xRef, wArr, f.Widths)
						}
					}
				}
			}
		}
		return f
	}

	// Single-byte Type1 / TrueType / Type3 font
	if encObj, ok := d.Find("Encoding"); ok {
		loadSingleByteEncoding(xRef, encObj, &f.Encoding)
	}
	firstChar := 0
	if fc := d.IntEntry("FirstChar"); fc != nil {
		firstChar = *fc
	}
	if wObj, ok := d.Find("Widths"); ok {
		if wArr, err := xRef.DereferenceArray(wObj); err == nil {
			for idx, item := range wArr {
				if w, ok := objectToFloat(xRef, item); ok {
					f.Widths[uint32(firstChar+idx)] = w
				}
			}
		}
	}
	return f
}

func loadSingleByteEncoding(xRef *model.XRefTable, encObj types.Object, tbl *singleByteEncoding) {
	resolved, err := xRef.Dereference(encObj)
	if err != nil {
		return
	}
	switch v := resolved.(type) {
	case types.Name:
		*tbl = initEncodingTable(string(v))
	case types.Dict:
		if base := v.NameEntry("BaseEncoding"); base != nil {
			*tbl = initEncodingTable(*base)
		}
		if diffObj, ok := v.Find("Differences"); ok {
			if arr, err := xRef.DereferenceArray(diffObj); err == nil {
				curCode := 0
				for _, item := range arr {
					obj, err := xRef.Dereference(item)
					if err != nil {
						continue
					}
					switch t := obj.(type) {
					case types.Integer:
						curCode = int(t)
					case types.Name:
						if curCode >= 0 && curCode < 256 {
							if r := resolveGlyphName(string(t)); r != 0 {
								tbl[curCode] = r
							}
						}
						curCode++
					}
				}
			}
		}
	}
}

func parseCIDWidths(xRef *model.XRefTable, arr types.Array, out map[uint32]float64) {
	for i := 0; i < len(arr); {
		cFirstF, ok := objectToFloat(xRef, arr[i])
		if !ok {
			i++
			continue
		}
		cFirst := uint32(cFirstF)
		if i+1 >= len(arr) {
			break
		}
		nextObj, err := xRef.Dereference(arr[i+1])
		if err != nil {
			i++
			continue
		}
		switch v := nextObj.(type) {
		case types.Array:
			for idx, wItem := range v {
				if w, ok := objectToFloat(xRef, wItem); ok {
					out[cFirst+uint32(idx)] = w
				}
			}
			i += 2
		default:
			if i+2 >= len(arr) {
				i += 2
				continue
			}
			cLastF, ok1 := objectToFloat(xRef, nextObj)
			wVal, ok2 := objectToFloat(xRef, arr[i+2])
			if ok1 && ok2 {
				cLast := uint32(cLastF)
				for c := cFirst; c <= cLast; c++ {
					out[c] = wVal
				}
			}
			i += 3
		}
	}
}

func numericEntry(xRef *model.XRefTable, d types.Dict, key string) (float64, bool) {
	obj, found := d.Find(key)
	if !found {
		return 0, false
	}
	return objectToFloat(xRef, obj)
}

func objectToFloat(xRef *model.XRefTable, obj types.Object) (float64, bool) {
	resolved, err := xRef.Dereference(obj)
	if err != nil {
		return 0, false
	}
	switch v := resolved.(type) {
	case types.Integer:
		return float64(v), true
	case types.Float:
		return float64(v), true
	default:
		_ = fmt.Sprint(v)
		return 0, false
	}
}
