package common

import (
	"github.com/dslipak/pdf"
)

func ExtractRowsFromPDF(path string) ([]string, error) {
	r, err := pdf.Open(path)
	// remember close file
	if err != nil {
		return nil, err
	}

	extracted_rows := []string{}
	
	for no := 1; no < r.NumPage(); no++ { // Loop over each page.
		page := r.Page(no)
		rows, _ := page.GetTextByRow()
		for _, row := range rows { // Loop over each row of text in the page.

			// concatenate all text in the row
			var rowText string
			for _, text := range row.Content {
				rowText += text.S + " "
			}

			// fmt.Println("Page", no, "Row", ri, "Text", rowText)
			extracted_rows = append(extracted_rows, rowText)
		}
	}

	// fmt.Println("Extracted rows: ", extracted_rows)

	return extracted_rows, nil
    // buf.ReadFrom(b)
	// return buf.String(), nil
}