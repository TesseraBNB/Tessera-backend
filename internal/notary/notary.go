// Package notary records Tessera verdicts on BNB Chain. Each verdict becomes a
// BNB Attestation Service (BAS) attestation — BAS is the BNB ecosystem's
// deployment of the Ethereum Attestation Service contracts — that is permanent,
// names the analysed project as its recipient, and carries keccak256 hashes of
// the report and of the evidence behind it, so anyone can later check that a
// report is unaltered. The report hash is also committed to Tessera's own
// TesseraAttestations contract, pointing back at the BAS attestation.
package notary

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Schema is the BAS schema every Tessera verdict is attested under.
const Schema = "address project,string subject,string kind,string verdict,bytes32 reportHash,bytes32 evidenceHash,string reportURI,string agent"

// Defaults for BNB Smart Chain testnet.
const (
	DefaultRPCURL = "https://bsc-testnet-rpc.publicnode.com"
	DefaultBAS    = "0x6c2270298b1e6046898a322acB3Cbad6F99f7CBD" // BAS (EAS v1.3.0) on BSC testnet
	// DefaultRegistry is Tessera's own TesseraAttestations contract on BSC testnet.
	DefaultRegistry = "0x56e6472693982df91df33842f1d087f2e4308427"
	DefaultChainID  = 97
	ExplorerURL     = "https://www.testnet.bascan.io"
	TxExplorerURL   = "https://testnet.bscscan.com/tx/"
)

const easABI = `[
{"type":"function","name":"attest","stateMutability":"payable","inputs":[{"name":"request","type":"tuple","components":[{"name":"schema","type":"bytes32"},{"name":"data","type":"tuple","components":[{"name":"recipient","type":"address"},{"name":"expirationTime","type":"uint64"},{"name":"revocable","type":"bool"},{"name":"refUID","type":"bytes32"},{"name":"data","type":"bytes"},{"name":"value","type":"uint256"}]}]}],"outputs":[{"name":"","type":"bytes32"}]},
{"type":"function","name":"getAttestation","stateMutability":"view","inputs":[{"name":"uid","type":"bytes32"}],"outputs":[{"name":"","type":"tuple","components":[{"name":"uid","type":"bytes32"},{"name":"schema","type":"bytes32"},{"name":"time","type":"uint64"},{"name":"expirationTime","type":"uint64"},{"name":"revocationTime","type":"uint64"},{"name":"refUID","type":"bytes32"},{"name":"recipient","type":"address"},{"name":"attester","type":"address"},{"name":"revocable","type":"bool"},{"name":"data","type":"bytes"}]}]},
{"type":"function","name":"getSchemaRegistry","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"address"}]},
{"type":"event","name":"Attested","anonymous":false,"inputs":[{"name":"recipient","type":"address","indexed":true},{"name":"attester","type":"address","indexed":true},{"name":"uid","type":"bytes32","indexed":false},{"name":"schemaUID","type":"bytes32","indexed":true}]}
]`

const registryABI = `[
{"type":"function","name":"register","stateMutability":"nonpayable","inputs":[{"name":"schema","type":"string"},{"name":"resolver","type":"address"},{"name":"revocable","type":"bool"}],"outputs":[{"name":"","type":"bytes32"}]},
{"type":"function","name":"getSchema","stateMutability":"view","inputs":[{"name":"uid","type":"bytes32"}],"outputs":[{"name":"","type":"tuple","components":[{"name":"uid","type":"bytes32"},{"name":"resolver","type":"address"},{"name":"revocable","type":"bool"},{"name":"schema","type":"string"}]}]}
]`

// tesseraABI is the part of TesseraAttestations the notary uses; its RiskLevel
// enum travels as uint8.
const tesseraABI = `[
{"type":"function","name":"commit","stateMutability":"nonpayable","inputs":[{"name":"verdictHash","type":"bytes32"},{"name":"riskLevel","type":"uint8"},{"name":"projectId","type":"string"},{"name":"evidenceUri","type":"string"}],"outputs":[]},
{"type":"function","name":"isNotarized","stateMutability":"view","inputs":[{"name":"verdictHash","type":"bytes32"}],"outputs":[{"name":"","type":"bool"}]}
]`

var (
	easParsed      = mustABI(easABI)
	registryParsed = mustABI(registryABI)
	tesseraParsed  = mustABI(tesseraABI)
	schemaArgs     = mustArgs(Schema)
)

// SchemaUID is the schema's id in the registry: keccak256(schema ‖ resolver ‖
// revocable), with no resolver and non-revocable attestations.
func SchemaUID() common.Hash {
	return crypto.Keccak256Hash([]byte(Schema), common.Address{}.Bytes(), []byte{0})
}

// Verdict is the content of one attestation.
type Verdict struct {
	Project      common.Address `json:"project"` // zero for proposal evaluations
	Subject      string         `json:"subject"` // the address or proposal name analysed
	Kind         string         `json:"kind"`    // "project-analysis" or "proposal-evaluation"
	Verdict      string         `json:"verdict"` // FUND, HOLD, REJECT or UNSPECIFIED
	ReportHash   common.Hash    `json:"reportHash"`
	EvidenceHash common.Hash    `json:"evidenceHash"`
	ReportURI    string         `json:"reportUri"`
	Agent        string         `json:"agent"`
}

// Receipt identifies a recorded attestation.
type Receipt struct {
	UID       string `json:"uid"`
	TxHash    string `json:"txHash"`
	Block     uint64 `json:"block"`
	ChainID   int64  `json:"chainId"`
	Attester  string `json:"attester"`
	SchemaUID string `json:"schemaUid"`
	Time      string `json:"time"`

	// the same report hash committed to TesseraAttestations
	Registry       string `json:"registry,omitempty"`
	RegistryTxHash string `json:"registryTxHash,omitempty"`
}

// Config selects the chain, contracts and signing key.
type Config struct {
	RPCURL     string
	PrivateKey string // hex, with or without 0x
	BAS        string
	Registry   string // TesseraAttestations; empty to skip
	ChainID    int64
}

// Notary signs attestations with one key; calls are serialised so nonces
// never collide.
type Notary struct {
	client  *ethclient.Client
	key     *ecdsa.PrivateKey
	from    common.Address
	bas     common.Address
	schemas common.Address // BAS schema registry
	tessera common.Address // TesseraAttestations; zero when disabled
	chainID *big.Int
	mu      sync.Mutex
}

// New connects to the chain. It returns nil, nil when no key is configured.
func New(ctx context.Context, cfg Config) (*Notary, error) {
	if strings.TrimSpace(cfg.PrivateKey) == "" {
		return nil, nil
	}
	key, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(cfg.PrivateKey), "0x"))
	if err != nil {
		return nil, errors.New("NOTARY_PRIVATE_KEY is not a valid secp256k1 key")
	}
	if !common.IsHexAddress(cfg.BAS) {
		return nil, fmt.Errorf("invalid BAS contract address %q", cfg.BAS)
	}
	if cfg.Registry != "" && !common.IsHexAddress(cfg.Registry) {
		return nil, fmt.Errorf("invalid TesseraAttestations address %q", cfg.Registry)
	}
	client, err := ethclient.DialContext(ctx, cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("notary RPC: %w", err)
	}
	n := &Notary{
		client:  client,
		key:     key,
		from:    crypto.PubkeyToAddress(key.PublicKey),
		bas:     common.HexToAddress(cfg.BAS),
		chainID: big.NewInt(cfg.ChainID),
	}
	if cfg.Registry != "" {
		n.tessera = common.HexToAddress(cfg.Registry)
	}
	out, err := n.call(ctx, easParsed, n.bas, "getSchemaRegistry")
	if err != nil {
		return nil, fmt.Errorf("BAS contract at %s: %w", cfg.BAS, err)
	}
	n.schemas = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	return n, nil
}

// Attester is the address that signs every attestation.
func (n *Notary) Attester() common.Address { return n.from }

// Contract is the BAS contract attestations are written to.
func (n *Notary) Contract() common.Address { return n.bas }

// Registry is the TesseraAttestations contract (zero when disabled).
func (n *Notary) Registry() common.Address { return n.tessera }

// RiskLevel maps a verdict to TesseraAttestations' RiskLevel enum:
// FUND → LOW (0), HOLD → MEDIUM (1), REJECT → HIGH (2), anything else → UNKNOWN (3).
func RiskLevel(verdict string) uint8 {
	switch verdict {
	case "FUND":
		return 0
	case "HOLD":
		return 1
	case "REJECT":
		return 2
	}
	return 3
}

// CommitToRegistry records reportHash in TesseraAttestations, pointing at the
// BAS attestation. It returns "" without a transaction when the registry is
// disabled or already holds the hash.
func (n *Notary) CommitToRegistry(ctx context.Context, reportHash common.Hash, verdict, projectID, evidenceURI string) (string, error) {
	if n.tessera == (common.Address{}) {
		return "", nil
	}
	out, err := n.call(ctx, tesseraParsed, n.tessera, "isNotarized", reportHash)
	if err != nil {
		return "", err
	}
	if done, _ := out[0].(bool); done {
		return "", nil
	}
	rec, err := n.transact(ctx, tesseraParsed, n.tessera, "commit", reportHash, RiskLevel(verdict), projectID, evidenceURI)
	if err != nil {
		return "", err
	}
	return rec.TxHash.Hex(), nil
}

// ChainID is the chain the notary writes to.
func (n *Notary) ChainID() int64 { return n.chainID.Int64() }

// Balance returns the attester's native balance in wei.
func (n *Notary) Balance(ctx context.Context) (*big.Int, error) {
	return n.client.BalanceAt(ctx, n.from, nil)
}

type schemaRecord struct {
	UID       [32]byte
	Resolver  common.Address
	Revocable bool
	Schema    string
}

// SchemaRegistered reports whether Tessera's schema exists in the registry.
func (n *Notary) SchemaRegistered(ctx context.Context) (bool, error) {
	out, err := n.call(ctx, registryParsed, n.schemas, "getSchema", SchemaUID())
	if err != nil {
		return false, err
	}
	rec := *abi.ConvertType(out[0], new(schemaRecord)).(*schemaRecord)
	return common.Hash(rec.UID) == SchemaUID(), nil
}

// RegisterSchema registers Tessera's schema (non-revocable, no resolver) and
// waits for the transaction. It is a no-op returning "" when already present.
func (n *Notary) RegisterSchema(ctx context.Context) (string, error) {
	if ok, err := n.SchemaRegistered(ctx); err != nil || ok {
		return "", err
	}
	rec, err := n.transact(ctx, registryParsed, n.schemas, "register", Schema, common.Address{}, false)
	if err != nil {
		return "", err
	}
	return rec.TxHash.Hex(), nil
}

// attestation request types, shaped for ABI packing
type requestData struct {
	Recipient      common.Address
	ExpirationTime uint64
	Revocable      bool
	RefUID         [32]byte
	Data           []byte
	Value          *big.Int
}

type attestRequest struct {
	Schema [32]byte
	Data   requestData
}

// Attest records v and waits for it to be mined.
func (n *Notary) Attest(ctx context.Context, v Verdict) (*Receipt, error) {
	data, err := EncodeData(v)
	if err != nil {
		return nil, err
	}
	req := attestRequest{
		Schema: SchemaUID(),
		Data:   requestData{Recipient: v.Project, Data: data, Value: big.NewInt(0)},
	}
	rec, err := n.transact(ctx, easParsed, n.bas, "attest", req)
	if err != nil {
		return nil, err
	}
	attested := easParsed.Events["Attested"].ID
	for _, l := range rec.Logs {
		if l.Address == n.bas && len(l.Topics) > 0 && l.Topics[0] == attested && len(l.Data) >= 32 {
			block, _ := n.client.HeaderByNumber(ctx, rec.BlockNumber)
			t := time.Now().UTC()
			if block != nil {
				t = time.Unix(int64(block.Time), 0).UTC()
			}
			return &Receipt{
				UID:       common.BytesToHash(l.Data[:32]).Hex(),
				TxHash:    rec.TxHash.Hex(),
				Block:     rec.BlockNumber.Uint64(),
				ChainID:   n.chainID.Int64(),
				Attester:  n.from.Hex(),
				SchemaUID: SchemaUID().Hex(),
				Time:      t.Format(time.RFC3339),
			}, nil
		}
	}
	return nil, fmt.Errorf("attestation tx %s has no Attested event", rec.TxHash.Hex())
}

// Attestation is a recorded attestation decoded with Tessera's schema.
type Attestation struct {
	UID       string  `json:"uid"`
	SchemaUID string  `json:"schemaUid"`
	Time      string  `json:"time"`
	Attester  string  `json:"attester"`
	Recipient string  `json:"recipient"`
	Verdict   Verdict `json:"verdict"`
}

// Get reads an attestation back from the chain.
func (n *Notary) Get(ctx context.Context, uid common.Hash) (*Attestation, error) {
	out, err := n.call(ctx, easParsed, n.bas, "getAttestation", uid)
	if err != nil {
		return nil, err
	}
	type onchain struct {
		UID            [32]byte
		Schema         [32]byte
		Time           uint64
		ExpirationTime uint64
		RevocationTime uint64
		RefUID         [32]byte
		Recipient      common.Address
		Attester       common.Address
		Revocable      bool
		Data           []byte
	}
	a := *abi.ConvertType(out[0], new(onchain)).(*onchain)
	if a.UID == ([32]byte{}) {
		return nil, fmt.Errorf("attestation %s not found", uid.Hex())
	}
	if common.Hash(a.Schema) != SchemaUID() {
		return nil, fmt.Errorf("attestation %s is not a Tessera verdict", uid.Hex())
	}
	v, err := DecodeData(a.Data)
	if err != nil {
		return nil, err
	}
	return &Attestation{
		UID:       uid.Hex(),
		SchemaUID: common.Hash(a.Schema).Hex(),
		Time:      time.Unix(int64(a.Time), 0).UTC().Format(time.RFC3339),
		Attester:  a.Attester.Hex(),
		Recipient: a.Recipient.Hex(),
		Verdict:   v,
	}, nil
}

// EncodeData ABI-encodes a verdict as the schema's attestation data.
func EncodeData(v Verdict) ([]byte, error) {
	return schemaArgs.Pack(v.Project, v.Subject, v.Kind, v.Verdict, [32]byte(v.ReportHash), [32]byte(v.EvidenceHash), v.ReportURI, v.Agent)
}

// DecodeData is the inverse of EncodeData.
func DecodeData(b []byte) (Verdict, error) {
	vals, err := schemaArgs.Unpack(b)
	if err != nil {
		return Verdict{}, fmt.Errorf("decode attestation data: %w", err)
	}
	return Verdict{
		Project:      vals[0].(common.Address),
		Subject:      vals[1].(string),
		Kind:         vals[2].(string),
		Verdict:      vals[3].(string),
		ReportHash:   common.Hash(vals[4].([32]byte)),
		EvidenceHash: common.Hash(vals[5].([32]byte)),
		ReportURI:    vals[6].(string),
		Agent:        vals[7].(string),
	}, nil
}

var (
	// "Verdict: Hold / Investigate", "**Verdict:** FUND", "Recommendation: HOLD / MONITOR"
	reVerdictCall = regexp.MustCompile(`(?i)(?:verdict|recommendation|decision)[ \t]*\**[ \t]*[:\-–—][ \t]*\**[ \t]*([a-z][a-z /'-]{1,40})`)
	// a line that opens with the call in bold: "**Fund.** rotki is…"
	reBoldCall = regexp.MustCompile(`(?im)^\s*\*\*\s*(do not fund|don't fund|reject|hold|investigate|fund)\b`)
	reReject   = regexp.MustCompile(`(?i)\b(reject|do not fund|don't fund|decline|defund)\b`)
	reHold     = regexp.MustCompile(`(?i)\b(hold|investigate|monitor|conditional)\b`)
	reFund     = regexp.MustCompile(`(?i)\bfund\b`)
)

// ExtractVerdict reads the call (FUND, HOLD or REJECT) from a report: the
// words right after "Verdict:", else a line that opens with the call in bold.
func ExtractVerdict(markdown string) string {
	for _, re := range []*regexp.Regexp{reVerdictCall, reBoldCall} {
		for _, m := range re.FindAllStringSubmatch(markdown, -1) {
			if call := classify(m[1]); call != "" {
				return call
			}
		}
	}
	return "UNSPECIFIED"
}

func classify(s string) string {
	switch {
	case reReject.MatchString(s):
		return "REJECT"
	case reHold.MatchString(s):
		return "HOLD"
	case reFund.MatchString(s):
		return "FUND"
	}
	return ""
}

// --- chain plumbing ---

func (n *Notary) call(ctx context.Context, parsed abi.ABI, to common.Address, method string, args ...any) ([]any, error) {
	var out []any
	err := bind.NewBoundContract(to, parsed, n.client, n.client, n.client).Call(&bind.CallOpts{Context: ctx}, &out, method, args...)
	return out, err
}

func (n *Notary) transact(ctx context.Context, parsed abi.ABI, to common.Address, method string, args ...any) (*types.Receipt, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	opts, err := bind.NewKeyedTransactorWithChainID(n.key, n.chainID)
	if err != nil {
		return nil, err
	}
	opts.Context = ctx
	tx, err := bind.NewBoundContract(to, parsed, n.client, n.client, n.client).Transact(opts, method, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	rec, err := bind.WaitMined(ctx, n.client, tx)
	if err != nil {
		return nil, fmt.Errorf("%s tx %s: %w", method, tx.Hash().Hex(), err)
	}
	if rec.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("%s tx %s reverted", method, tx.Hash().Hex())
	}
	return rec, nil
}

func mustABI(s string) abi.ABI {
	a, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(err)
	}
	return a
}

// mustArgs turns "type name,type name" into ABI arguments.
func mustArgs(schema string) abi.Arguments {
	var args abi.Arguments
	for _, field := range strings.Split(schema, ",") {
		parts := strings.Fields(field)
		t, err := abi.NewType(parts[0], "", nil)
		if err != nil {
			panic(err)
		}
		args = append(args, abi.Argument{Name: parts[1], Type: t})
	}
	return args
}
