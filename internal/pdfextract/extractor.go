package pdfextract

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// ExtractFile parses a PDF file from disk using pdfcpu and extracts
// positioned text spans and clustered visual rows for every page.
func ExtractFile(path string, password string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf %q: %w", path, err)
	}
	defer f.Close()

	return ExtractReader(context.Background(), f, path, password)
}

// ExtractReader parses a PDF from an io.ReadSeeker and extracts layout-aware pages.
func ExtractReader(c context.Context, rs io.ReadSeeker, sourceName string, password string) (*Document, error) {
	conf := model.NewStatelessConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	if password != "" {
		conf.UserPW = password
		conf.OwnerPW = password
	}

	ctx, err := api.ReadContext(c, rs, conf)
	if err != nil {
		return nil, fmt.Errorf("read pdf %q: %w", sourceName, err)
	}
	if err := ctx.EnsurePageCount(); err != nil {
		return nil, fmt.Errorf("count pages %q: %w", sourceName, err)
	}

	doc := &Document{
		Path:  sourceName,
		Pages: make([]*Page, 0, ctx.PageCount),
	}

	for pageNr := 1; pageNr <= ctx.PageCount; pageNr++ {
		_, _, inh, err := ctx.PageDict(c, pageNr, true)
		if err != nil {
			return nil, fmt.Errorf("page %d dict %q: %w", pageNr, sourceName, err)
		}

		box := types.NewRectangle(0, 0, 595, 842)
		var resources types.Dict
		if inh != nil {
			if inh.CropBox != nil {
				box = inh.CropBox
			} else if inh.MediaBox != nil {
				box = inh.MediaBox
			}
			resources = inh.Resources
		}

		rd, err := pdfcpu.ExtractPageContent(c, ctx, pageNr)
		if err != nil {
			return nil, fmt.Errorf("page %d content %q: %w", pageNr, sourceName, err)
		}
		var streamBytes []byte
		if rd != nil {
			streamBytes, err = io.ReadAll(rd)
			if err != nil {
				return nil, fmt.Errorf("read page %d stream %q: %w", pageNr, sourceName, err)
			}
		}

		interp := &streamInterpreter{
			xRef:        ctx.XRefTable,
			decodeLimit: conf.DecodeLimit(),
			box:         box,
		}
		interp.run(streamBytes, resources, defaultGraphicsState(), 0)

		page := &Page{
			Number: pageNr,
			Width:  box.Width(),
			Height: box.Height(),
			Spans:  interp.spans,
			Rows:   buildVisualRows(interp.spans),
		}
		doc.Pages = append(doc.Pages, page)
	}

	return doc, nil
}
