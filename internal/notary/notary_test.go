package notary

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// The UID the BAS schema registry on BSC testnet holds for Schema
// (registered by `tessera notary-setup`, cross-checked with `cast`).
const registeredUID = "0xcd4d38906641353fefefe1caabcba23f730b0512039c1b3c5478d47cf97373f8"

func TestSchemaUIDMatchesRegistry(t *testing.T) {
	if got := SchemaUID().Hex(); got != registeredUID {
		t.Fatalf("SchemaUID = %s, registry has %s — changing Schema needs a new registration", got, registeredUID)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	v := Verdict{
		Project:      common.HexToAddress("0x9531c059098e3d194ff87febb587ab07b30b1306"),
		Subject:      "0x9531c059098e3d194ff87febb587ab07b30b1306",
		Kind:         "project-analysis",
		Verdict:      "FUND",
		ReportHash:   crypto.Keccak256Hash([]byte("# Report")),
		EvidenceHash: crypto.Keccak256Hash([]byte("[]")),
		ReportURI:    "https://example.org/api/reports/x.md",
		Agent:        "qwen/qwen3.8-omni-flash:free via Tessera",
	}
	b, err := EncodeData(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeData(b)
	if err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Fatalf("round trip changed the verdict:\n got %+v\nwant %+v", got, v)
	}
}

func TestExtractVerdict(t *testing.T) {
	cases := []struct{ md, want string }{
		{"## Summary Verdict\n**Verdict: Hold / Investigate Further Confidence:** Medium", "HOLD"},
		{"## Summary Verdict\n**Verdict: FUND**\nrotki demonstrates…", "FUND"},
		{"**Verdict:** REJECT — the donor base is a single whale", "REJECT"},
		{"Verdict: Do not fund until the team ships", "REJECT"},
		{"## Summary Verdict\n**Recommendation: HOLD / MONITOR**", "HOLD"},
		// the heading's next line is a bullet: must not be read as "Verdict - …"
		{"## Summary Verdict\n- **Gitcoin Grants:** $165k donated\n\n**Fund.** rotki is a cornerstone", "FUND"},
		{"Public goods funding matters, but no call is made here.", "UNSPECIFIED"},
	}
	for _, c := range cases {
		if got := ExtractVerdict(c.md); got != c.want {
			t.Errorf("ExtractVerdict(%q) = %s, want %s", c.md, got, c.want)
		}
	}
}
