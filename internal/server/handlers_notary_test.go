package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/yeheskieltame/tessera/internal/config"
)

// inTempDir runs the test with the working directory (and so reports/) in a
// fresh temporary directory.
func inTempDir(t *testing.T) {
	t.Helper()
	wd, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestSaveRunHashesExactFilesAndDetectsTampering(t *testing.T) {
	inTempDir(t)
	a := &App{}
	md := "# rotki\n\n**Verdict: FUND**\n"
	ev := []evidenceItem{{Tool: "find_in_gitcoin", Input: json.RawMessage(`{"address":"0x95"}`), Result: `{"match":"address"}`}}
	rec, err := a.saveRun(runMeta{Title: "Project Analysis: 0x95", Kind: "project-analysis", Subject: "0x95"},
		"reports/project_analysis__0x95_20261002_080000.pdf", md, ev, "api.xkiro.com", "qwen")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != "project_analysis__0x95_20261002_080000" || rec.Verdict != "FUND" || rec.ToolCalls != 1 {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.ReportHash != crypto.Keccak256Hash([]byte(md)).Hex() {
		t.Fatal("report hash is not keccak256 of the report text")
	}
	evBytes, _ := os.ReadFile(filepath.Join(reportsDir, rec.ID+".evidence.json"))
	if rec.EvidenceHash != crypto.Keccak256Hash(evBytes).Hex() {
		t.Fatal("evidence hash is not keccak256 of the evidence file")
	}
	loaded, err := loadRun(rec.ID)
	if err != nil || loaded.ReportHash != rec.ReportHash {
		t.Fatalf("loadRun: %+v %v", loaded, err)
	}
	if err := checkRunFiles(loaded); err != nil {
		t.Fatalf("untouched files should pass: %v", err)
	}
	_ = os.WriteFile(filepath.Join(reportsDir, rec.ID+".md"), []byte(md+"edited"), 0o644)
	if err := checkRunFiles(loaded); err == nil {
		t.Fatal("an edited report must fail the hash check")
	}
}

func TestNotarizeRefusesWithoutNotaryOrValidID(t *testing.T) {
	inTempDir(t)
	a := &App{cfg: &config.Config{}}
	for _, c := range []struct {
		query string
		code  int
	}{
		{"?id=project_analysis__0x95_1", http.StatusServiceUnavailable}, // no notary configured
	} {
		rec := httptest.NewRecorder()
		a.handleNotarize(rec, httptest.NewRequest(http.MethodPost, "/api/notarize"+c.query, nil))
		if rec.Code != c.code {
			t.Errorf("%s: status %d, want %d", c.query, rec.Code, c.code)
		}
	}
	if reRunID.MatchString("../etc/passwd") || reRunID.MatchString("a/b") || !reRunID.MatchString("project_analysis__0x95_20261002_080000") {
		t.Fatal("run id pattern must reject paths and accept report ids")
	}
}
