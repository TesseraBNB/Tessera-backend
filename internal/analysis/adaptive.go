package analysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/yeheskieltame/tessera/internal/data"
)

// SignalGap represents a missing or weak signal that could be filled.
type SignalGap struct {
	Source      string // e.g. "oso", "github", "blockchain", "temporal", "gitcoin"
	Reason      string // why this gap matters
	Priority    string // "high", "medium", "low"
	Recoverable bool   // can we attempt to fill this gap?
}

// AdaptiveResult holds the outcome of adaptive collection.
type AdaptiveResult struct {
	GapsDetected  []SignalGap
	GapsFilled    []string // descriptions of gaps that were filled
	GapsRemaining []string // descriptions of gaps that couldn't be filled
	ExtraOSO      string   // additional OSO data collected
	ExtraGitHub   string   // additional GitHub data collected
	ExtraChain    string   // additional blockchain data collected
	ExtraTemporal string   // additional temporal analysis
	ExtraGitcoin  string   // additional Gitcoin data collected
	Iterations    int      // how many collection rounds were needed
}

// CollectedSignals represents all signals gathered so far in the pipeline.
type CollectedSignals struct {
	Address       string
	HasHistory    bool
	HistoryEpochs int
	HasTrust      bool
	TrustProfile  *TrustProfile
	HasChainData  bool
	ChainSignals  *data.ChainSignals
	HasOSO        bool
	OSOMetrics    string
	HasGitHub     bool
	GitHubSignals string
	HasAnomalies  bool
	AnomalyCount  int
	OSOName       string // if provided by user
	GitHubURL     string // if discovered
}

// AssessGaps analyzes collected signals and identifies what's missing or weak.
func AssessGaps(signals *CollectedSignals) []SignalGap {
	var gaps []SignalGap

	// Gap: No OSO data but we have an address (could discover project name)
	if !signals.HasOSO && signals.OSOName == "" {
		gaps = append(gaps, SignalGap{
			Source:      "oso",
			Reason:      "No OSO ecosystem metrics — cannot assess code activity, on-chain usage, or cross-platform funding",
			Priority:    "high",
			Recoverable: true,
		})
	}

	// Gap: No GitHub data
	if !signals.HasGitHub {
		gaps = append(gaps, SignalGap{
			Source:      "github",
			Reason:      "No GitHub repository data — cannot assess code quality, team activity, or development momentum",
			Priority:    "medium",
			Recoverable: true, // can try to discover from OSO or chain data
		})
	}

	// Gap: Chain data shows no activity (might need more chains or different address format)
	if signals.HasChainData && signals.ChainSignals != nil && signals.ChainSignals.TotalChainsActive == 0 {
		gaps = append(gaps, SignalGap{
			Source:      "blockchain",
			Reason:      "Address has no on-chain activity across all scanned chains — legitimacy signal missing",
			Priority:    "high",
			Recoverable: false,
		})
	}

	// Gap: Trust profile shows high coordination risk but no temporal analysis
	if signals.HasTrust && signals.TrustProfile != nil {
		if signals.TrustProfile.CoordinationRisk > 0.5 && !signals.HasAnomalies {
			gaps = append(gaps, SignalGap{
				Source:      "temporal",
				Reason:      fmt.Sprintf("High coordination risk (%.2f) detected but no temporal analysis — cannot determine if this is a new or persistent pattern", signals.TrustProfile.CoordinationRisk),
				Priority:    "high",
				Recoverable: true,
			})
		}
		if signals.TrustProfile.WhaleDepRatio > 0.5 && !signals.HasAnomalies {
			gaps = append(gaps, SignalGap{
				Source:      "temporal",
				Reason:      fmt.Sprintf("High whale dependency (%.0f%%) — temporal analysis needed to check if whale is consistent or new entrant", signals.TrustProfile.WhaleDepRatio*100),
				Priority:    "high",
				Recoverable: true,
			})
		}
	}

	// Gap: Only 1 epoch of history — no trend analysis possible
	if signals.HasHistory && signals.HistoryEpochs < 2 {
		gaps = append(gaps, SignalGap{
			Source:      "temporal",
			Reason:      "Single epoch of data — no trend or trajectory analysis possible",
			Priority:    "medium",
			Recoverable: false,
		})
	}

	// Gap: No Gitcoin cross-reference
	if signals.HasChainData && signals.ChainSignals != nil && signals.ChainSignals.TotalChainsActive > 0 {
		gaps = append(gaps, SignalGap{
			Source:      "gitcoin",
			Reason:      "No Gitcoin Grants cross-reference — cannot assess broader funding ecosystem presence",
			Priority:    "low",
			Recoverable: true,
		})
	}

	// Gap: Low donor diversity but no deeper investigation
	if signals.HasTrust && signals.TrustProfile != nil && signals.TrustProfile.DonorDiversity < 0.3 && signals.TrustProfile.UniqueDonors > 1 {
		gaps = append(gaps, SignalGap{
			Source:      "donor_deep",
			Reason:      fmt.Sprintf("Very low donor diversity (%.2f) with %d donors — deeper investigation needed into donor behavior patterns", signals.TrustProfile.DonorDiversity, signals.TrustProfile.UniqueDonors),
			Priority:    "medium",
			Recoverable: false,
		})
	}

	return gaps
}

// Sources are the upstream clients adaptive collection may query.
type Sources struct {
	OSO     *data.OSOClient
	Gitcoin *data.GitcoinClient
}

// AdaptiveCollect attempts to fill identified signal gaps by collecting additional data.
// It runs up to maxIterations rounds: assess → collect → re-assess.
func AdaptiveCollect(ctx context.Context, src Sources, signals *CollectedSignals, maxIterations int) *AdaptiveResult {
	result := &AdaptiveResult{}

	for iter := 0; iter < maxIterations; iter++ {
		gaps := AssessGaps(signals)
		if len(gaps) == 0 {
			break
		}

		if iter == 0 {
			result.GapsDetected = gaps
		}
		result.Iterations = iter + 1

		filled := false

		for _, gap := range gaps {
			if !gap.Recoverable {
				result.GapsRemaining = append(result.GapsRemaining, fmt.Sprintf("[%s] %s", gap.Source, gap.Reason))
				continue
			}

			switch gap.Source {
			case "oso":
				if tryFillOSO(ctx, src, signals, result) {
					filled = true
				}
			case "github":
				if tryFillGitHub(ctx, signals, result) {
					filled = true
				}
			case "gitcoin":
				if tryFillGitcoin(ctx, src, signals, result) {
					filled = true
				}
			default:
				result.GapsRemaining = append(result.GapsRemaining, fmt.Sprintf("[%s] %s", gap.Source, gap.Reason))
			}
		}

		if !filled {
			// No new data collected, stop iterating
			break
		}
	}

	// Final gap assessment
	remaining := AssessGaps(signals)
	for _, gap := range remaining {
		desc := fmt.Sprintf("[%s/%s] %s", gap.Source, gap.Priority, gap.Reason)
		found := false
		for _, r := range result.GapsRemaining {
			if r == desc {
				found = true
				break
			}
		}
		if !found {
			result.GapsRemaining = append(result.GapsRemaining, desc)
		}
	}

	return result
}

// tryFillOSO resolves the project in Open Source Observer, from its address or
// its GitHub repository, and collects its signals.
func tryFillOSO(ctx context.Context, src Sources, signals *CollectedSignals, result *AdaptiveResult) bool {
	if signals.HasOSO || src.OSO == nil || !src.OSO.Enabled() {
		return false
	}

	var project *data.OSOProject
	how := ""
	if signals.Address != "" && src.Gitcoin != nil {
		if p, err := src.Gitcoin.OSOProjectForAddress(ctx, signals.Address); err == nil && p != nil {
			project, how = p, "its address"
		}
	}
	if project == nil && signals.GitHubURL != "" {
		if owner, repo, err := data.ParseGitHubURL(signals.GitHubURL); err == nil {
			for _, term := range []string{repo, owner} {
				if p, _, err := src.OSO.FindProject(ctx, term); err == nil && p != nil {
					project, how = p, fmt.Sprintf("GitHub name '%s'", term)
					break
				}
			}
		}
	}
	if project == nil {
		return false
	}

	s, err := src.OSO.Signals(ctx, project)
	if err != nil {
		return false
	}
	formatted := s.FormatSignals()
	if formatted == "No OSO data available for this project." {
		return false
	}
	signals.HasOSO = true
	signals.OSOMetrics = formatted
	signals.OSOName = project.Name
	result.ExtraOSO = formatted
	result.GapsFilled = append(result.GapsFilled, fmt.Sprintf("Discovered OSO project '%s' via %s", project.Name, how))
	return true
}

// tryFillGitHub attempts to discover GitHub repo from OSO data or other signals.
func tryFillGitHub(ctx context.Context, signals *CollectedSignals, result *AdaptiveResult) bool {
	if signals.HasGitHub {
		return false
	}

	// Strategy 1: If we have OSO data with a project name, try it as a GitHub org/repo
	if signals.OSOName != "" {
		gh := data.NewGitHubClient()
		// Many OSO projects use their GitHub org name as project name
		parts := strings.SplitN(signals.OSOName, "/", 2)
		var owner, repo string
		if len(parts) == 2 {
			owner, repo = parts[0], parts[1]
		} else {
			// Try as both org/same-name
			owner = signals.OSOName
			repo = signals.OSOName
		}

		ghSignals := gh.CollectGitHubSignals(ctx, owner, repo)
		if ghSignals.Repo != nil {
			formatted := ghSignals.FormatSignals()
			signals.HasGitHub = true
			signals.GitHubSignals = formatted
			signals.GitHubURL = fmt.Sprintf("https://github.com/%s/%s", owner, repo)
			result.ExtraGitHub = formatted
			result.GapsFilled = append(result.GapsFilled,
				fmt.Sprintf("Discovered GitHub repo %s/%s from OSO project name", owner, repo))
			return true
		}
	}

	return false
}

// tryFillGitcoin looks the project's address up in Gitcoin Grants.
func tryFillGitcoin(ctx context.Context, src Sources, signals *CollectedSignals, result *AdaptiveResult) bool {
	if signals.Address == "" || src.Gitcoin == nil || src.OSO == nil || !src.OSO.Enabled() {
		return false
	}
	p, err := src.Gitcoin.FindByAddress(ctx, signals.Address)
	if err != nil || p == nil {
		return false
	}
	info := fmt.Sprintf("Found in Gitcoin Grants as '%s': %d rounds, %d unique donors, $%.0f donated + $%.0f matched",
		p.Name, p.RoundCount, p.UniqueDonors, p.DonatedUSD, p.MatchedUSD)
	result.ExtraGitcoin = info
	result.GapsFilled = append(result.GapsFilled, "Cross-referenced with Gitcoin Grants: "+info)
	return true
}

// FormatAdaptiveResult produces a markdown summary of the adaptive collection.
func FormatAdaptiveResult(r *AdaptiveResult) string {
	if r == nil || (len(r.GapsFilled) == 0 && len(r.GapsRemaining) == 0) {
		return ""
	}

	var b strings.Builder
	b.WriteString("### Adaptive Signal Collection\n\n")
	b.WriteString(fmt.Sprintf("**Collection rounds:** %d | **Gaps detected:** %d | **Gaps filled:** %d\n\n",
		r.Iterations, len(r.GapsDetected), len(r.GapsFilled)))

	if len(r.GapsFilled) > 0 {
		b.WriteString("**Signals recovered:**\n")
		for _, g := range r.GapsFilled {
			b.WriteString(fmt.Sprintf("- %s\n", g))
		}
		b.WriteString("\n")
	}

	if len(r.GapsRemaining) > 0 {
		b.WriteString("**Remaining gaps (not recoverable):**\n")
		for _, g := range r.GapsRemaining {
			b.WriteString(fmt.Sprintf("- %s\n", g))
		}
		b.WriteString("\n")
	}

	if r.ExtraOSO != "" {
		b.WriteString("**Discovered OSO Data:**\n" + r.ExtraOSO + "\n")
	}
	if r.ExtraGitHub != "" {
		b.WriteString("**Discovered GitHub Data:**\n" + r.ExtraGitHub + "\n")
	}
	if r.ExtraGitcoin != "" {
		b.WriteString("**Discovered Gitcoin Data:**\n" + r.ExtraGitcoin + "\n")
	}

	return b.String()
}
