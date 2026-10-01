package report

import (
	"bytes"
	"os"
	"testing"
)

func TestGeneratePDFWritesValidFile(t *testing.T) {
	t.Chdir(t.TempDir()) // GeneratePDF writes under ./reports

	path, err := GeneratePDF(&PDFReport{
		Title:    "Project Analysis: 0xabc",
		Subtitle: "Tessera — Public Goods Intelligence",
		Model:    "test-model",
		Provider: "test",
		Metadata: map[string]string{"Generated": "2026-10-01T00:00:00Z"},
		Sections: []PDFSection{{Heading: "Report", Body: "## Summary verdict\n\n**Hold** — whale dependency 45.8%."}},
	})
	if err != nil {
		t.Fatalf("GeneratePDF: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Errorf("output does not start with a PDF header: %q", b[:min(len(b), 8)])
	}
}
