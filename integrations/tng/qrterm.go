package tng

import (
	"io"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// The portal's QR encodes the init response's deeplink verbatim (checked by
// decoding the portal's PNG, October 2026), so the terminal QR is generated
// from the deeplink. go-qrcode packs the deeplink's long digit runs in numeric
// mode, and a screen needs only Low error correction: 33 modules instead of
// the 41 a byte-only encoder at Medium produces.

// qrQuietZone is the white margin in modules; scanners need some, and the
// terminal around it may be dark.
const qrQuietZone = 2

// Half blocks with explicit colours (top half foreground, bottom half
// background), so the code scans on dark and light terminal themes alike.
func qrCell(top, bottom bool) string {
	c := func(dark bool) string {
		if dark {
			return "16" // black
		}
		return "231" // white
	}
	return "\x1b[38;5;" + c(top) + "m\x1b[48;5;" + c(bottom) + "m▀"
}

// WriteTerminalQR prints the login QR to w, two module rows per line.
func (q *QRLogin) WriteTerminalQR(w io.Writer) bool {
	if q.Deeplink == "" {
		return false
	}
	code, err := qrcode.New(q.Deeplink, qrcode.Low)
	if err != nil {
		return false
	}
	code.DisableBorder = true
	grid := code.Bitmap()
	n := len(grid) + 2*qrQuietZone
	dark := func(r, c int) bool {
		r, c = r-qrQuietZone, c-qrQuietZone
		return r >= 0 && c >= 0 && r < len(grid) && c < len(grid) && grid[r][c]
	}
	var sb strings.Builder
	for r := 0; r < n; r += 2 {
		for c := 0; c < n; c++ {
			sb.WriteString(qrCell(dark(r, c), dark(r+1, c)))
		}
		sb.WriteString("\x1b[0m\n")
	}
	_, err = io.WriteString(w, sb.String())
	return err == nil
}
