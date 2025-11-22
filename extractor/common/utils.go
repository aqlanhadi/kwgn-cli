package common

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/dslipak/pdf"
)

// ExtractRowsFromPDFReader reads a PDF from an io.Reader and returns text rows
func ExtractRowsFromPDFReader(reader io.Reader) (*[]string, error) {
	var rAt io.ReaderAt
	var size int64

	switch v := reader.(type) {
	case io.ReaderAt:
		rAt = v
		if seeker, ok := reader.(io.Seeker); ok {
			cur, _ := seeker.Seek(0, io.SeekCurrent)
			end, _ := seeker.Seek(0, io.SeekEnd)
			seeker.Seek(cur, io.SeekStart)
			size = end
		} else {
			return nil, errors.New("reader is io.ReaderAt but not io.Seeker, cannot determine size")
		}
	default:
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(reader); err != nil {
			return nil, err
		}
		b := buf.Bytes()
		rAt = bytes.NewReader(b)
		size = int64(len(b))
	}

	r, err := pdf.NewReader(rAt, size)
	if err != nil {
		return nil, err
	}

	numPages := r.NumPage()
	extractedRows := make([]string, 0, numPages*50)

	for no := 1; no <= numPages; no++ {
		page := r.Page(no)
		rows, err := page.GetTextByRow()
		if err != nil {
			log.Printf("Warning: error getting text from page %d: %v", no, err)
			continue
		}

		for _, row := range rows {
			var builder strings.Builder
			for i, text := range row.Content {
				builder.WriteString(text.S)
				if i < len(row.Content)-1 {
					builder.WriteByte(' ')
				}
			}
			if builder.Len() > 0 {
				extractedRows = append(extractedRows, builder.String())
			}
		}
	}

	return &extractedRows, nil
}

// ExtractRowsFromPDF reads a PDF file and returns text rows
func ExtractRowsFromPDF(path string) (*[]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ExtractRowsFromPDFReader(file)
}
