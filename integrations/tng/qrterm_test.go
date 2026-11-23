package tng

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalQRFromDeeplink(t *testing.T) {
	q := &QRLogin{Deeplink: "qrl://034a90d6-5eb2-4078-86d4-f336f294d958,202610151115115001171959850719956,353532363136"}
	var b bytes.Buffer
	if !q.WriteTerminalQR(&b) {
		t.Fatal("no QR written")
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	// This deeplink fits a 33-module code; with the quiet zone that is 37
	// rows, two per line. A larger code means the compact encoding was lost.
	if len(lines) != 19 {
		t.Errorf("%d lines, want 19", len(lines))
	}
	if !strings.Contains(b.String(), qrCell(true, false)) {
		t.Error("QR drawn without the fixed colours")
	}
	if (&QRLogin{}).WriteTerminalQR(&b) {
		t.Error("wrote a QR with no deeplink")
	}
}
