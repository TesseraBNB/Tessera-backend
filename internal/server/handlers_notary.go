package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/yeheskieltame/tessera/internal/notary"
)

// Every titled agent run leaves three files in reports/, named after its PDF:
//
//	<id>.md            the report exactly as the reader received it
//	<id>.evidence.json every tool call with its input and raw result
//	<id>.run.json      metadata, both keccak256 hashes, and the notary receipt
//
// The hashes are of those exact bytes, so anyone who downloads the files can
// recompute them and compare with the attestation on BNB Chain.

// runMeta describes an agent run; runs without a Title produce no report.
type runMeta struct {
	Title   string
	Kind    string // "project-analysis" or "proposal-evaluation"
	Subject string // the project address or proposal name
}

type evidenceItem struct {
	Tool    string          `json:"tool"`
	Input   json.RawMessage `json:"input,omitempty"`
	Result  string          `json:"result"`
	IsError bool            `json:"isError,omitempty"`
}

type runRecord struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Kind         string          `json:"kind"`
	Subject      string          `json:"subject"`
	Model        string          `json:"model"`
	Provider     string          `json:"provider"`
	CreatedAt    string          `json:"createdAt"`
	Verdict      string          `json:"verdict"`
	ReportHash   string          `json:"reportHash"`
	EvidenceHash string          `json:"evidenceHash"`
	ToolCalls    int             `json:"toolCalls"`
	Notary       *notary.Receipt `json:"notary,omitempty"`
}

const reportsDir = "reports"

var reRunID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`)

// saveRun writes a run's report, evidence and record. pdfPath names the run;
// without one the id is derived from the kind and time.
func (a *App) saveRun(meta runMeta, pdfPath, md string, evidence []evidenceItem, provider, model string) (*runRecord, error) {
	id := strings.TrimSuffix(filepath.Base(pdfPath), ".pdf")
	if pdfPath == "" || !reRunID.MatchString(id) {
		id = strings.ReplaceAll(meta.Kind, "-", "_") + "_" + time.Now().UTC().Format("20060102_150405")
	}
	if evidence == nil {
		evidence = []evidenceItem{}
	}
	ev, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return nil, err
	}
	rec := &runRecord{
		ID:           id,
		Title:        meta.Title,
		Kind:         meta.Kind,
		Subject:      meta.Subject,
		Model:        model,
		Provider:     provider,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		Verdict:      notary.ExtractVerdict(md),
		ReportHash:   crypto.Keccak256Hash([]byte(md)).Hex(),
		EvidenceHash: crypto.Keccak256Hash(ev).Hex(),
		ToolCalls:    len(evidence),
	}
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(reportsDir, id+".md"), []byte(md), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(reportsDir, id+".evidence.json"), ev, 0o644); err != nil {
		return nil, err
	}
	return rec, writeRun(rec)
}

func writeRun(rec *runRecord) error {
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(reportsDir, rec.ID+".run.json"), b, 0o644)
}

func loadRun(id string) (*runRecord, error) {
	b, err := os.ReadFile(filepath.Join(reportsDir, id+".run.json"))
	if err != nil {
		return nil, err
	}
	var rec runRecord
	return &rec, json.Unmarshal(b, &rec)
}

// checkRunFiles recomputes both hashes from the files on disk.
func checkRunFiles(rec *runRecord) error {
	for _, f := range []struct{ name, want string }{
		{rec.ID + ".md", rec.ReportHash},
		{rec.ID + ".evidence.json", rec.EvidenceHash},
	} {
		b, err := os.ReadFile(filepath.Join(reportsDir, f.name))
		if err != nil {
			return err
		}
		if got := crypto.Keccak256Hash(b).Hex(); got != f.want {
			return fmt.Errorf("%s changed since the run (hash %s, recorded %s)", f.name, got, f.want)
		}
	}
	return nil
}

type notaryInfo struct {
	Enabled   bool   `json:"enabled"`
	ChainID   int64  `json:"chainId"`
	Contract  string `json:"contract"`
	RPCURL    string `json:"rpcUrl"`
	Schema    string `json:"schema"`
	SchemaUID string `json:"schemaUid"`
	Attester  string `json:"attester,omitempty"`
	Explorer  string `json:"explorer"`
}

func (a *App) handleNotaryInfo(w http.ResponseWriter, r *http.Request) {
	info := notaryInfo{
		ChainID:   a.cfg.NotaryChainID,
		Contract:  a.cfg.NotaryBAS,
		RPCURL:    a.cfg.NotaryRPCURL,
		Schema:    notary.Schema,
		SchemaUID: notary.SchemaUID().Hex(),
		Explorer:  notary.ExplorerURL,
	}
	if a.notary != nil {
		info.Enabled, info.Attester = true, a.notary.Attester().Hex()
	}
	a.writeJSON(w, http.StatusOK, info)
}

type notarizeResponse struct {
	*notary.Receipt
	AttestationURL string `json:"attestationUrl"`
	TxURL          string `json:"txUrl"`
	ReportURI      string `json:"reportUri"`
	ReportHash     string `json:"reportHash"`
	EvidenceHash   string `json:"evidenceHash"`
	Verdict        string `json:"verdict"`
}

// handleNotarize attests one stored run on BNB Chain. It only accepts runs this
// server produced (by id), re-checks their hashes first, and returns the
// existing receipt when the run was already notarised.
func (a *App) handleNotarize(w http.ResponseWriter, r *http.Request) {
	if a.notary == nil {
		a.jsonError(w, "verdict notary is not configured on this server", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if !reRunID.MatchString(id) {
		a.jsonError(w, "id must be a report id from an agent run", http.StatusBadRequest)
		return
	}

	a.notaryMu.Lock()
	defer a.notaryMu.Unlock()

	rec, err := loadRun(id)
	if errors.Is(err, os.ErrNotExist) {
		a.jsonError(w, "no run with this id on this server", http.StatusNotFound)
		return
	}
	if err != nil {
		a.jsonError(w, "run record unreadable: "+err.Error(), http.StatusInternalServerError)
		return
	}
	reportURI := a.cfg.PublicURL + "/api/reports/" + rec.ID + ".md"
	if rec.Notary != nil {
		a.writeJSON(w, http.StatusOK, a.notarizeResponse(rec, reportURI))
		return
	}
	if err := checkRunFiles(rec); err != nil {
		a.jsonError(w, "refusing to notarise: "+err.Error(), http.StatusConflict)
		return
	}
	if !a.notaryBudget.allow() {
		a.jsonError(w, "daily notarisation budget exhausted — try again tomorrow", http.StatusTooManyRequests)
		return
	}

	v := notary.Verdict{
		Subject:      rec.Subject,
		Kind:         rec.Kind,
		Verdict:      rec.Verdict,
		ReportHash:   common.HexToHash(rec.ReportHash),
		EvidenceHash: common.HexToHash(rec.EvidenceHash),
		ReportURI:    reportURI,
		Agent:        rec.Model + " via Tessera",
	}
	if common.IsHexAddress(rec.Subject) {
		v.Project = common.HexToAddress(rec.Subject)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	receipt, err := a.notary.Attest(ctx, v)
	if err != nil {
		a.log.Error("notarisation failed", "id", rec.ID, "error", err.Error())
		a.jsonError(w, "notarisation failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	rec.Notary = receipt
	if err := writeRun(rec); err != nil {
		a.log.Error("notary receipt not saved", "id", rec.ID, "uid", receipt.UID, "error", err.Error())
	}
	a.log.Info("verdict notarised", "id", rec.ID, "uid", receipt.UID, "tx", receipt.TxHash)
	a.writeJSON(w, http.StatusOK, a.notarizeResponse(rec, reportURI))
}

func (a *App) notarizeResponse(rec *runRecord, reportURI string) notarizeResponse {
	return notarizeResponse{
		Receipt:        rec.Notary,
		AttestationURL: notary.ExplorerURL + "/attestation/" + rec.Notary.UID,
		TxURL:          notary.TxExplorerURL + rec.Notary.TxHash,
		ReportURI:      reportURI,
		ReportHash:     rec.ReportHash,
		EvidenceHash:   rec.EvidenceHash,
		Verdict:        rec.Verdict,
	}
}
