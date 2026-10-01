package analysis

import (
	"fmt"
	"math"
	"strings"

	"github.com/yeheskieltame/tessera/internal/data"
)

// CorroborationVerdict classifies how well signals agree.
type CorroborationVerdict string

const (
	VerdictConfirmed    CorroborationVerdict = "CONFIRMED"    // sources agree
	VerdictConflicting  CorroborationVerdict = "CONFLICTING"  // sources disagree significantly
	VerdictPartial      CorroborationVerdict = "PARTIAL"      // partial agreement
	VerdictUnverifiable CorroborationVerdict = "UNVERIFIABLE" // only one source available
)

// CorroborationCheck represents one cross-verification between sources.
type CorroborationCheck struct {
	Claim       string               `json:"claim"`
	SourceA     string               `json:"sourceA"`
	ValueA      string               `json:"valueA"`
	SourceB     string               `json:"sourceB"`
	ValueB      string               `json:"valueB"`
	Verdict     CorroborationVerdict `json:"verdict"`
	Explanation string               `json:"explanation"`
	Severity    string               `json:"severity"` // "high", "medium", "low"
}

// CorroborationReport aggregates all cross-verification checks.
type CorroborationReport struct {
	Checks         []CorroborationCheck `json:"checks"`
	ConfirmedCount int                  `json:"confirmedCount"`
	ConflictCount  int                  `json:"conflictCount"`
	PartialCount   int                  `json:"partialCount"`
	TrustScore     float64              `json:"trustScore"` // 0-100, weighted by severity
}

// CrossVerifySignals compares signals across multiple independent sources.
func CrossVerifySignals(
	trust *TrustProfile,
	chain *data.ChainSignals,
	osoSignals *data.ProjectSignals,
	githubSignals *data.GitHubSignals,
	history []data.ProjectEpochData,
) *CorroborationReport {
	report := &CorroborationReport{}

	// Check 1: OSO contributors (all repos, all time) vs GitHub's top-30 list for one repo.
	// OSO should cover at least as many people; a much smaller count means the
	// OSO project and the repo are probably not the same thing.
	if osoSignals != nil && osoSignals.Code != nil && githubSignals != nil && githubSignals.Repo != nil {
		osoContribs := osoSignals.Code.Contributors
		ghContribs := math.Max(float64(len(githubSignals.Contributors)), 1)
		ratio := osoContribs / ghContribs

		verdict := VerdictConfirmed
		explanation := fmt.Sprintf("OSO counts %.0f contributors across the project's repos; GitHub lists %.0f on this repo (ratio %.2f)", osoContribs, ghContribs, ratio)
		severity := "low"
		if ratio < 0.4 {
			verdict = VerdictConflicting
			explanation += " — OSO sees far fewer people than this one repo has, the two may be different projects"
			severity = "medium"
		} else if ratio < 0.7 {
			verdict = VerdictPartial
			explanation += " — OSO has somewhat fewer, likely an indexing lag"
		}

		report.Checks = append(report.Checks, CorroborationCheck{
			Claim:       "Contributor count",
			SourceA:     "OSO (all repos, all time)",
			ValueA:      fmt.Sprintf("%.0f contributors", osoContribs),
			SourceB:     "GitHub API (top 30, one repo)",
			ValueB:      fmt.Sprintf("%.0f contributors", ghContribs),
			Verdict:     verdict,
			Explanation: explanation,
			Severity:    severity,
		})
	}

	// Check 2: OSO stars (summed over the project's repos) vs this repo's stars.
	if osoSignals != nil && osoSignals.Code != nil && githubSignals != nil && githubSignals.Repo != nil {
		osoStars := osoSignals.Code.Stars
		ghStars := float64(githubSignals.Repo.Stars)

		verdict := VerdictConfirmed
		explanation := fmt.Sprintf("OSO: %.0f stars across %.0f repos, GitHub: %.0f on this repo", osoStars, osoSignals.Code.Repositories, ghStars)
		severity := "low"
		if ghStars > 0 && osoStars < ghStars*0.5 {
			verdict = VerdictConflicting
			explanation += " — the project total is below one repo's count, investigate the mapping or data freshness"
			severity = "medium"
		} else if ghStars > 0 && osoStars < ghStars*0.8 {
			verdict = VerdictPartial
			explanation += " — OSO slightly behind, likely indexing lag"
		}

		report.Checks = append(report.Checks, CorroborationCheck{
			Claim:       "Repository star count",
			SourceA:     "OSO (all repos)",
			ValueA:      fmt.Sprintf("%.0f stars", osoStars),
			SourceB:     "GitHub API (real-time)",
			ValueB:      fmt.Sprintf("%d stars", githubSignals.Repo.Stars),
			Verdict:     verdict,
			Explanation: explanation,
			Severity:    severity,
		})
	}

	// Check 4: Funding consistency (Octant allocated vs matched ratio)
	if len(history) > 0 {
		var totalAlloc, totalMatched float64
		for _, h := range history {
			totalAlloc += h.Allocated
			totalMatched += h.Matched
		}

		if totalAlloc > 0 {
			matchRatio := totalMatched / totalAlloc
			verdict := VerdictConfirmed
			explanation := fmt.Sprintf("Total allocated: %.4f ETH, matched: %.4f ETH, match ratio: %.2fx", totalAlloc, totalMatched, matchRatio)
			severity := "medium"

			if matchRatio > 10 {
				verdict = VerdictPartial
				explanation += " — extremely high match ratio suggests small donor count with large matching pool amplification"
				severity = "medium"
			}

			report.Checks = append(report.Checks, CorroborationCheck{
				Claim:       "Funding match ratio consistency",
				SourceA:     "Octant (allocated by donors)",
				ValueA:      fmt.Sprintf("%.4f ETH direct", totalAlloc),
				SourceB:     "Octant (protocol matching)",
				ValueB:      fmt.Sprintf("%.4f ETH matched (%.2fx)", totalMatched, matchRatio),
				Verdict:     verdict,
				Explanation: explanation,
				Severity:    severity,
			})
		}
	}

	// Check 5: Donor diversity vs whale dependency (internal consistency)
	if trust != nil {
		verdict := VerdictConfirmed
		explanation := fmt.Sprintf("Diversity: %.3f, Whale dep: %.1f%%", trust.DonorDiversity, trust.WhaleDepRatio*100)
		severity := "low"

		// High diversity + high whale dep = internal contradiction
		if trust.DonorDiversity > 0.7 && trust.WhaleDepRatio > 0.5 {
			verdict = VerdictConflicting
			explanation += " — high diversity score but high whale dependency is mathematically unusual, check donor count"
			severity = "high"
		}
		// Low diversity + low whale dep = unusual but possible (many small equal donors)
		if trust.DonorDiversity < 0.3 && trust.WhaleDepRatio < 0.2 {
			verdict = VerdictPartial
			explanation += " — low diversity but low whale ratio suggests few donors contributing equal amounts"
		}

		report.Checks = append(report.Checks, CorroborationCheck{
			Claim:       "Donor diversity vs whale dependency consistency",
			SourceA:     "Trust Graph (Shannon entropy)",
			ValueA:      fmt.Sprintf("%.3f diversity", trust.DonorDiversity),
			SourceB:     "Trust Graph (whale ratio)",
			ValueB:      fmt.Sprintf("%.1f%% whale dependency", trust.WhaleDepRatio*100),
			Verdict:     verdict,
			Explanation: explanation,
			Severity:    severity,
		})
	}

	// Check 6: Octant funding as OSO records it (USD) vs Octant's own history (ETH).
	// The implied ETH price should be plausible; otherwise one side is missing epochs.
	if osoSignals != nil && osoSignals.FundingUSD["OCTANT"] > 0 && len(history) > 0 {
		osoOctant := osoSignals.FundingUSD["OCTANT"]
		var octantETH float64
		for _, h := range history {
			octantETH += h.Allocated + h.Matched
		}

		verdict := VerdictConfirmed
		severity := "low"
		explanation := fmt.Sprintf("OSO records $%.0f from Octant; the Octant API shows %.2f ETH", osoOctant, octantETH)
		if octantETH > 0 {
			implied := osoOctant / octantETH
			explanation += fmt.Sprintf(" (implied $%.0f/ETH)", implied)
			if implied < 800 || implied > 6000 {
				verdict = VerdictConflicting
				explanation += " — outside any plausible ETH price, one source is missing or double-counting epochs"
				severity = "medium"
			}
		}

		report.Checks = append(report.Checks, CorroborationCheck{
			Claim:       "Octant funding total",
			SourceA:     "OSO (Octant, all time, USD)",
			ValueA:      fmt.Sprintf("$%.0f USD", osoOctant),
			SourceB:     "Octant API (all epochs)",
			ValueB:      fmt.Sprintf("%.4f ETH", octantETH),
			Verdict:     verdict,
			Explanation: explanation,
			Severity:    severity,
		})
	}

	// Check 7: GitHub activity vs code deployment (has contracts?)
	if githubSignals != nil && githubSignals.Repo != nil && chain != nil {
		hasCode := !githubSignals.Repo.Archived && githubSignals.Repo.Size > 0
		hasContracts := chain.HasContracts

		verdict := VerdictUnverifiable
		explanation := "Checking if GitHub code activity aligns with on-chain deployment"
		severity := "low"

		if hasCode && hasContracts {
			verdict = VerdictConfirmed
			explanation = fmt.Sprintf("Active GitHub repo (%s, %d KB) with deployed contracts — code-to-chain consistency confirmed",
				githubSignals.Repo.Language, githubSignals.Repo.Size)
		} else if hasCode && !hasContracts && chain.TotalChainsActive > 0 {
			verdict = VerdictPartial
			explanation = "Active code repo but no deployed contracts — project may be off-chain or contracts under different address"
		} else if !hasCode && hasContracts {
			verdict = VerdictConflicting
			explanation = "Deployed contracts but archived/empty GitHub repo — possible abandoned project with live contracts"
			severity = "high"
		}

		report.Checks = append(report.Checks, CorroborationCheck{
			Claim:       "Code activity vs on-chain deployment",
			SourceA:     "GitHub",
			ValueA:      fmt.Sprintf("Active: %v, Language: %s, Size: %d KB", hasCode, githubSignals.Repo.Language, githubSignals.Repo.Size),
			SourceB:     "Blockchain Scan",
			ValueB:      fmt.Sprintf("Contracts: %v, Active chains: %d", hasContracts, chain.TotalChainsActive),
			Verdict:     verdict,
			Explanation: explanation,
			Severity:    severity,
		})
	}

	// Compute summary
	for _, c := range report.Checks {
		switch c.Verdict {
		case VerdictConfirmed:
			report.ConfirmedCount++
		case VerdictConflicting:
			report.ConflictCount++
		case VerdictPartial:
			report.PartialCount++
		}
	}

	// Trust score: weighted by severity
	if len(report.Checks) > 0 {
		totalWeight := 0.0
		score := 0.0
		for _, c := range report.Checks {
			w := 1.0
			switch c.Severity {
			case "high":
				w = 3.0
			case "medium":
				w = 2.0
			}
			totalWeight += w
			switch c.Verdict {
			case VerdictConfirmed:
				score += w * 1.0
			case VerdictPartial:
				score += w * 0.6
			case VerdictUnverifiable:
				score += w * 0.5
			case VerdictConflicting:
				score += w * 0.0
			}
		}
		report.TrustScore = (score / totalWeight) * 100
	}

	return report
}

// FormatCorroborationReport produces markdown output.
func FormatCorroborationReport(r *CorroborationReport) string {
	if r == nil || len(r.Checks) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("### Signal Corroboration (Cross-Verification)\n\n")
	b.WriteString(fmt.Sprintf("**Trust Score:** %.0f/100 | **Checks:** %d total | %d confirmed | %d conflicting | %d partial\n\n",
		r.TrustScore, len(r.Checks), r.ConfirmedCount, r.ConflictCount, r.PartialCount))

	for i, c := range r.Checks {
		icon := "?"
		switch c.Verdict {
		case VerdictConfirmed:
			icon = "OK"
		case VerdictConflicting:
			icon = "!!"
		case VerdictPartial:
			icon = "~"
		}
		b.WriteString(fmt.Sprintf("**%d. %s** [%s] (%s severity)\n", i+1, c.Claim, icon, c.Severity))
		b.WriteString(fmt.Sprintf("- %s: %s\n", c.SourceA, c.ValueA))
		b.WriteString(fmt.Sprintf("- %s: %s\n", c.SourceB, c.ValueB))
		b.WriteString(fmt.Sprintf("- **Verdict:** %s — %s\n\n", c.Verdict, c.Explanation))
	}

	return b.String()
}
