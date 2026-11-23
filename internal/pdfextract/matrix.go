// Package pdfextract implements pure-Go, layout-aware PDF text and coordinate
// extraction supporting PDF 1.0-1.7 (including xref streams, object streams,
// Type0/CID fonts, ToUnicode CMaps, and CTM/Tm coordinate normalization).
package pdfextract

import "math"

// Matrix represents a 2D PDF affine transformation matrix in row-vector form:
//
//	[ A  B  0 ]
//	[ C  D  0 ]
//	[ E  F  1 ]
//
// Transforming point (x, y) yields:
//
//	x' = A*x + C*y + E
//	y' = B*x + D*y + F
type Matrix struct {
	A, B, C, D, E, F float64
}

// Identity returns the 2D identity transformation matrix.
func Identity() Matrix {
	return Matrix{A: 1, D: 1}
}

// Translate returns a translation matrix shifting by (tx, ty).
func Translate(tx, ty float64) Matrix {
	return Matrix{A: 1, D: 1, E: tx, F: ty}
}

// Mul computes m1 * m2 in row-vector multiplication order, so that
// Mul(m1, m2).Apply(x, y) equals m2.Apply(m1.Apply(x, y)).
func Mul(m1, m2 Matrix) Matrix {
	return Matrix{
		A: m1.A*m2.A + m1.B*m2.C,
		B: m1.A*m2.B + m1.B*m2.D,
		C: m1.C*m2.A + m1.D*m2.C,
		D: m1.C*m2.B + m1.D*m2.D,
		E: m1.E*m2.A + m1.F*m2.C + m2.E,
		F: m1.E*m2.B + m1.F*m2.D + m2.F,
	}
}

// Apply transforms point (x, y) by m.
func (m Matrix) Apply(x, y float64) (float64, float64) {
	return m.A*x + m.C*y + m.E, m.B*x + m.D*y + m.F
}

// ScaleY returns the magnitude of the transformed Y basis vector (0, 1).
func (m Matrix) ScaleY() float64 {
	return math.Hypot(m.C, m.D)
}

// ScaleX returns the magnitude of the transformed X basis vector (1, 0).
func (m Matrix) ScaleX() float64 {
	return math.Hypot(m.A, m.B)
}
