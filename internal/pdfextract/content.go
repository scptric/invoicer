package pdfextract

import (
	"math"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// TextSpan is a single positioned text run extracted from a PDF content stream.
// Y is top-down (0 at top of the page box, increasing downward).
type TextSpan struct {
	Text     string
	X        float64
	Y        float64
	EndX     float64
	FontSize float64
	Bold     bool
	FontName string
}

type graphicsState struct {
	ctm      Matrix
	font     *Font
	fontName string
	fontSize float64
	tc       float64 // character spacing
	tw       float64 // word spacing
	th       float64 // horizontal scaling (percentage, default 100)
	tl       float64 // text leading
	trise    float64 // text rise
}

func defaultGraphicsState() graphicsState {
	return graphicsState{
		ctm: Identity(),
		th:  100.0,
	}
}

type streamInterpreter struct {
	xRef        *model.XRefTable
	decodeLimit int64
	box         *types.Rectangle
	spans       []TextSpan
}

func (si *streamInterpreter) run(streamBytes []byte, resources types.Dict, state graphicsState, depth int) {
	if depth > 8 || len(streamBytes) == 0 {
		return
	}
	fonts := loadPageFonts(si.xRef, resources, si.decodeLimit)
	tokens := tokenizeContentStream(streamBytes)

	var stack []graphicsState
	var operands []token
	tm := Identity()
	tlm := Identity()
	inText := false

	popNum := func(idx int) (float64, bool) {
		if idx < 0 || idx >= len(operands) {
			return 0, false
		}
		if operands[idx].kind == tokNumber {
			return operands[idx].num, true
		}
		return 0, false
	}

	showBytes := func(raw []byte) {
		if !inText || len(raw) == 0 {
			return
		}
		txt, width1000, charCount, spaceCount := state.font.Decode(raw)
		fs := state.fontSize
		th := state.th / 100.0
		if th == 0 {
			th = 1.0
		}
		advance := ((width1000/1000.0)*fs + float64(charCount)*state.tc + float64(spaceCount)*state.tw) * th

		trmStart := Mul(tm, state.ctm)
		sx, sy := trmStart.Apply(0, state.trise)

		tm = Mul(Translate(advance, 0), tm)
		trmEnd := Mul(tm, state.ctm)
		ex, _ := trmEnd.Apply(0, state.trise)

		if strings.TrimSpace(txt) == "" {
			return
		}

		topY := si.box.UR.Y - sy
		normX := sx - si.box.LL.X
		normEndX := ex - si.box.LL.X
		if normEndX < normX {
			normEndX = normX
		}
		effFontSize := math.Abs(fs) * trmStart.ScaleY()
		if effFontSize == 0 {
			effFontSize = math.Abs(fs)
		}
		bold := false
		if state.font != nil {
			bold = state.font.Bold
		}

		si.spans = append(si.spans, TextSpan{
			Text:     txt,
			X:        normX,
			Y:        topY,
			EndX:     normEndX,
			FontSize: effFontSize,
			Bold:     bold,
			FontName: state.fontName,
		})
	}

	for _, tok := range tokens {
		if tok.kind != tokOperator {
			operands = append(operands, tok)
			continue
		}

		op := tok.str
		switch op {
		case "q":
			stack = append(stack, state)
		case "Q":
			if len(stack) > 0 {
				state = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case "cm":
			if len(operands) >= 6 {
				a, ok1 := popNum(len(operands) - 6)
				b, ok2 := popNum(len(operands) - 5)
				c, ok3 := popNum(len(operands) - 4)
				d, ok4 := popNum(len(operands) - 3)
				e, ok5 := popNum(len(operands) - 2)
				f, ok6 := popNum(len(operands) - 1)
				if ok1 && ok2 && ok3 && ok4 && ok5 && ok6 {
					m := Matrix{A: a, B: b, C: c, D: d, E: e, F: f}
					state.ctm = Mul(m, state.ctm)
				}
			}
		case "BT":
			inText = true
			tm = Identity()
			tlm = Identity()
		case "ET":
			inText = false
		case "Tf":
			if len(operands) >= 2 {
				nameTok := operands[len(operands)-2]
				size, ok := popNum(len(operands) - 1)
				if nameTok.kind == tokName && ok {
					state.fontName = nameTok.str
					state.font = fonts[nameTok.str]
					state.fontSize = size
				}
			}
		case "Tc":
			if v, ok := popNum(len(operands) - 1); ok {
				state.tc = v
			}
		case "Tw":
			if v, ok := popNum(len(operands) - 1); ok {
				state.tw = v
			}
		case "Tz":
			if v, ok := popNum(len(operands) - 1); ok {
				state.th = v
			}
		case "TL":
			if v, ok := popNum(len(operands) - 1); ok {
				state.tl = v
			}
		case "Ts":
			if v, ok := popNum(len(operands) - 1); ok {
				state.trise = v
			}
		case "Tm":
			if len(operands) >= 6 {
				a, ok1 := popNum(len(operands) - 6)
				b, ok2 := popNum(len(operands) - 5)
				c, ok3 := popNum(len(operands) - 4)
				d, ok4 := popNum(len(operands) - 3)
				e, ok5 := popNum(len(operands) - 2)
				f, ok6 := popNum(len(operands) - 1)
				if ok1 && ok2 && ok3 && ok4 && ok5 && ok6 {
					tm = Matrix{A: a, B: b, C: c, D: d, E: e, F: f}
					tlm = tm
				}
			}
		case "Td":
			if len(operands) >= 2 {
				tx, ok1 := popNum(len(operands) - 2)
				ty, ok2 := popNum(len(operands) - 1)
				if ok1 && ok2 {
					tlm = Mul(Translate(tx, ty), tlm)
					tm = tlm
				}
			}
		case "TD":
			if len(operands) >= 2 {
				tx, ok1 := popNum(len(operands) - 2)
				ty, ok2 := popNum(len(operands) - 1)
				if ok1 && ok2 {
					state.tl = -ty
					tlm = Mul(Translate(tx, ty), tlm)
					tm = tlm
				}
			}
		case "T*":
			tlm = Mul(Translate(0, -state.tl), tlm)
			tm = tlm
		case "Tj":
			if len(operands) >= 1 {
				last := operands[len(operands)-1]
				if last.kind == tokString || last.kind == tokHexString {
					showBytes(last.raw)
				}
			}
		case "'":
			tlm = Mul(Translate(0, -state.tl), tlm)
			tm = tlm
			if len(operands) >= 1 {
				last := operands[len(operands)-1]
				if last.kind == tokString || last.kind == tokHexString {
					showBytes(last.raw)
				}
			}
		case "\"":
			if len(operands) >= 3 {
				if tw, ok := popNum(len(operands) - 3); ok {
					state.tw = tw
				}
				if tc, ok := popNum(len(operands) - 2); ok {
					state.tc = tc
				}
				tlm = Mul(Translate(0, -state.tl), tlm)
				tm = tlm
				last := operands[len(operands)-1]
				if last.kind == tokString || last.kind == tokHexString {
					showBytes(last.raw)
				}
			}
		case "TJ":
			if len(operands) >= 1 && operands[len(operands)-1].kind == tokArray {
				th := state.th / 100.0
				if th == 0 {
					th = 1.0
				}
				for _, elem := range operands[len(operands)-1].arr {
					switch elem.kind {
					case tokString, tokHexString:
						showBytes(elem.raw)
					case tokNumber:
						tx := -(elem.num / 1000.0) * state.fontSize * th
						tm = Mul(Translate(tx, 0), tm)
					}
				}
			}
		case "Do":
			if len(operands) >= 1 && operands[len(operands)-1].kind == tokName && resources != nil {
				si.execFormXObject(operands[len(operands)-1].str, resources, state, depth)
			}
		}

		operands = operands[:0]
	}
}

func (si *streamInterpreter) execFormXObject(name string, resources types.Dict, parentState graphicsState, depth int) {
	xoObj, found := resources.Find("XObject")
	if !found {
		return
	}
	xoDict, err := si.xRef.DereferenceDict(xoObj)
	if err != nil || xoDict == nil {
		return
	}
	entry, ok := xoDict.Find(name)
	if !ok {
		return
	}
	sd, _, err := si.xRef.DereferenceStreamDict(entry)
	if err != nil || sd == nil {
		return
	}
	st := sd.Subtype()
	if st == nil || *st != "Form" {
		return
	}
	if err := sd.DecodeWithLimit(si.decodeLimit); err != nil {
		return
	}

	childState := parentState
	if matObj, ok := sd.Find("Matrix"); ok {
		if arr, err := si.xRef.DereferenceArray(matObj); err == nil && len(arr) == 6 {
			var vals [6]float64
			allOK := true
			for i := 0; i < 6; i++ {
				v, ok := objectToFloat(si.xRef, arr[i])
				if !ok {
					allOK = false
					break
				}
				vals[i] = v
			}
			if allOK {
				formMat := Matrix{A: vals[0], B: vals[1], C: vals[2], D: vals[3], E: vals[4], F: vals[5]}
				childState.ctm = Mul(formMat, childState.ctm)
			}
		}
	}

	formRes := resources
	if resObj, ok := sd.Find("Resources"); ok {
		if rd, err := si.xRef.DereferenceDict(resObj); err == nil && rd != nil {
			formRes = rd
		}
	}

	si.run(sd.Content, formRes, childState, depth+1)
}
