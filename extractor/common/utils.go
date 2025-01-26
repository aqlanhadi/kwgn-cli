package common

import (
	"bytes"

	"github.com/dslipak/pdf"
)

func ExtractTextFromPDF(path string) (string, error) {
	f, err := pdf.Open(path)
	// remember close file
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
    b, err := f.GetPlainText()
    if err != nil {
        return "", err
    }
    buf.ReadFrom(b)
	return buf.String(), nil
}