package data

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Gitcoin Grants data, read from Open Source Observer's copy of the Gitcoin
// dataset: every donation and matching payout from 2019 until Gitcoin shut
// Grants Stack down in May 2025 (its own indexer at grants-stack-indexer-v2
// went offline with it). Needs OSO_API_KEY.
const (
	gitcoinTable    = "oso.int_events__gitcoin_funding"
	GitcoinCoverage = "Gitcoin Grants Stack rounds, 2019 to May 2025 (GG23), via Open Source Observer"
)

type GitcoinClient struct {
	oso *OSOClient
}

func NewGitcoinClient(oso *OSOClient) *GitcoinClient {
	return &GitcoinClient{oso: oso}
}

// GitcoinRound is one round, or one project's funding within a round.
type GitcoinRound struct {
	Ref          string  `json:"ref"` // chainId:roundId, how tools and the CLI name a round
	RoundID      string  `json:"roundId"`
	RoundName    string  `json:"roundName"`
	RoundNumber  int     `json:"roundNumber,omitempty"` // GG number; 0 for community rounds
	Chain        string  `json:"chain,omitempty"`
	ChainID      int     `json:"chainId,omitempty"`
	Projects     int     `json:"projects,omitempty"`
	Donations    int     `json:"donations"`
	UniqueDonors int     `json:"uniqueDonors"`
	DonatedUSD   float64 `json:"donatedUsd"`
	MatchedUSD   float64 `json:"matchedUsd"`
	LastDonation string  `json:"lastDonation,omitempty"`
}

// GitcoinProject is a funding recipient, with its per-round history when
// looked up by address.
type GitcoinProject struct {
	RecipientAddress string         `json:"recipientAddress"`
	Name             string         `json:"name"`
	OSOProject       string         `json:"osoProject,omitempty"`
	RoundCount       int            `json:"roundCount"`
	Donations        int            `json:"donations"`
	UniqueDonors     int            `json:"uniqueDonors"`
	DonatedUSD       float64        `json:"donatedUsd"`
	MatchedUSD       float64        `json:"matchedUsd"`
	Rounds           []GitcoinRound `json:"rounds,omitempty"`
}

// GitcoinDonation is one donor-to-project contribution in a round.
type GitcoinDonation struct {
	Donor     string
	Recipient string
	Project   string
	AmountUSD float64
}

// per-group aggregate columns shared by the queries below
const gitcoinAgg = `count_if(event_source = 'GITCOIN_DONATIONS') AS donations,
	count(DISTINCT donor_address) AS donors,
	sum(CASE WHEN event_source = 'GITCOIN_DONATIONS' THEN amount_in_usd ELSE 0 END) AS donated,
	sum(CASE WHEN event_source = 'GITCOIN_MATCHING' THEN amount_in_usd ELSE 0 END) AS matched`

// FindByAddress returns everything a recipient address received, round by
// round (newest first), or nil when it never received Gitcoin funding.
func (g *GitcoinClient) FindByAddress(ctx context.Context, addr string) (*GitcoinProject, error) {
	lit, err := sqlAddress(addr)
	if err != nil {
		return nil, err
	}
	// GROUPING SETS adds an all-rounds total row (round_id NULL) so unique
	// donors are counted once across rounds.
	q := `SELECT round_id, max(round_name) AS round_name, max(round_number) AS round_number,
	max(chain) AS chain, max(chain_id) AS chain_id, ` + gitcoinAgg + `,
	max(gitcoin_group_project_name) AS name, max(oso_project_name) AS oso,
	CAST(max(time) AS varchar) AS last_time, count(DISTINCT round_id) AS rounds
	FROM ` + gitcoinTable + ` WHERE recipient_address = ` + lit + `
	GROUP BY GROUPING SETS ((round_id, chain_id), ())`
	rows, err := g.oso.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	var p *GitcoinProject
	var rounds []GitcoinRound
	for _, r := range rows {
		if r["round_id"] == nil { // the total row
			if rowInt(r, "rounds") == 0 {
				continue
			}
			p = &GitcoinProject{
				RecipientAddress: strings.ToLower(addr),
				Name:             rowStr(r, "name"),
				OSOProject:       rowStr(r, "oso"),
				RoundCount:       rowInt(r, "rounds"),
				Donations:        rowInt(r, "donations"),
				UniqueDonors:     rowInt(r, "donors"),
				DonatedUSD:       round2(rowNum(r, "donated")),
				MatchedUSD:       round2(rowNum(r, "matched")),
			}
			continue
		}
		rounds = append(rounds, roundFromRow(r))
	}
	if p == nil {
		return nil, nil
	}
	sortRoundsNewest(rounds)
	p.Rounds = rounds
	return p, nil
}

// SearchByName finds recipients whose Gitcoin project name contains name.
// Names are not unique, so these are candidates, not proof of identity.
func (g *GitcoinClient) SearchByName(ctx context.Context, name string, limit int) ([]GitcoinProject, error) {
	lit, err := sqlName(name)
	if err != nil {
		return nil, err
	}
	q := `SELECT recipient_address, max(gitcoin_group_project_name) AS name, max(oso_project_name) AS oso,
	count(DISTINCT round_id) AS rounds, ` + gitcoinAgg + `
	FROM ` + gitcoinTable + ` WHERE strpos(lower(gitcoin_group_project_name), ` + lit + `) > 0
	GROUP BY recipient_address ORDER BY sum(amount_in_usd) DESC LIMIT ` + strconv.Itoa(clampLimit(limit, 10))
	rows, err := g.oso.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]GitcoinProject, 0, len(rows))
	for _, r := range rows {
		out = append(out, projectFromRow(r))
	}
	return out, nil
}

// Rounds lists the most recent rounds with their totals.
func (g *GitcoinClient) Rounds(ctx context.Context, limit int) ([]GitcoinRound, error) {
	q := `SELECT round_id, max(round_name) AS round_name, max(round_number) AS round_number,
	max(chain) AS chain, chain_id, count(DISTINCT recipient_address) AS projects, ` + gitcoinAgg + `,
	CAST(max(time) AS varchar) AS last_time
	FROM ` + gitcoinTable + ` GROUP BY round_id, chain_id ORDER BY max(time) DESC LIMIT ` + strconv.Itoa(clampLimit(limit, 50))
	rows, err := g.oso.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]GitcoinRound, 0, len(rows))
	for _, r := range rows {
		out = append(out, roundFromRow(r))
	}
	return out, nil
}

// RoundProjects returns a round's summary and its recipients ranked by total
// funding (donations + matching). ref is chainId:roundId.
func (g *GitcoinClient) RoundProjects(ctx context.Context, ref string) (*GitcoinRound, []GitcoinProject, error) {
	where, err := sqlRoundFilter(ref)
	if err != nil {
		return nil, nil, err
	}
	q := `SELECT recipient_address, max(round_name) AS round_name, max(round_number) AS round_number,
	max(chain) AS chain, max(chain_id) AS chain_id, max(gitcoin_group_project_name) AS name, max(oso_project_name) AS oso,
	count(DISTINCT round_id) AS rounds, ` + gitcoinAgg + `, CAST(max(time) AS varchar) AS last_time
	FROM ` + gitcoinTable + ` WHERE ` + where + `
	GROUP BY GROUPING SETS ((recipient_address), ())`
	rows, err := g.oso.Query(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	var summary *GitcoinRound
	var projects []GitcoinProject
	for _, r := range rows {
		if r["recipient_address"] == nil {
			if rowInt(r, "rounds") > 0 {
				s := roundFromRow(r)
				s.Ref, s.RoundID = ref, ref[strings.Index(ref, ":")+1:]
				summary = &s
			}
			continue
		}
		projects = append(projects, projectFromRow(r))
	}
	if summary == nil {
		return nil, nil, fmt.Errorf("round %s not found in the Gitcoin dataset", ref)
	}
	summary.Projects = len(projects)
	sortProjectsByFunding(projects)
	return summary, projects, nil
}

// RoundDonations returns every individual donation in a round (matching
// payouts have no donor and are left out). ref is chainId:roundId.
func (g *GitcoinClient) RoundDonations(ctx context.Context, ref string) ([]GitcoinDonation, error) {
	where, err := sqlRoundFilter(ref)
	if err != nil {
		return nil, err
	}
	q := `SELECT donor_address, recipient_address, gitcoin_group_project_name AS name, amount_in_usd
	FROM ` + gitcoinTable + ` WHERE ` + where + ` AND event_source = 'GITCOIN_DONATIONS' AND donor_address IS NOT NULL`
	rows, err := g.oso.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]GitcoinDonation, 0, len(rows))
	for _, r := range rows {
		out = append(out, GitcoinDonation{
			Donor:     rowStr(r, "donor_address"),
			Recipient: rowStr(r, "recipient_address"),
			Project:   rowStr(r, "name"),
			AmountUSD: rowNum(r, "amount_in_usd"),
		})
	}
	return out, nil
}

// OSOProjectForAddress resolves an address to the OSO project that lists it:
// first through Gitcoin's recipient-to-project mapping (fast), then OSO's
// artifact registry (a large table, slow). It returns nil when nothing matches.
func (g *GitcoinClient) OSOProjectForAddress(ctx context.Context, addr string) (*OSOProject, error) {
	p, err := g.FindByAddress(ctx, addr)
	if err != nil {
		return nil, err
	}
	if p != nil && p.OSOProject != "" {
		if proj, _, err := g.oso.FindProject(ctx, p.OSOProject); err == nil && proj != nil {
			return proj, nil
		}
	}
	return g.oso.FindProjectByAddress(ctx, addr)
}

// --- row mapping ---

func roundFromRow(r map[string]any) GitcoinRound {
	ref := ""
	if id := rowStr(r, "round_id"); id != "" {
		ref = strconv.Itoa(rowInt(r, "chain_id")) + ":" + id
	}
	return GitcoinRound{
		Ref:          ref,
		RoundID:      rowStr(r, "round_id"),
		RoundName:    rowStr(r, "round_name"),
		RoundNumber:  rowInt(r, "round_number"),
		Chain:        rowStr(r, "chain"),
		ChainID:      rowInt(r, "chain_id"),
		Projects:     rowInt(r, "projects"),
		Donations:    rowInt(r, "donations"),
		UniqueDonors: rowInt(r, "donors"),
		DonatedUSD:   round2(rowNum(r, "donated")),
		MatchedUSD:   round2(rowNum(r, "matched")),
		LastDonation: rowStr(r, "last_time"),
	}
}

func projectFromRow(r map[string]any) GitcoinProject {
	return GitcoinProject{
		RecipientAddress: rowStr(r, "recipient_address"),
		Name:             rowStr(r, "name"),
		OSOProject:       rowStr(r, "oso"),
		RoundCount:       rowInt(r, "rounds"),
		Donations:        rowInt(r, "donations"),
		UniqueDonors:     rowInt(r, "donors"),
		DonatedUSD:       round2(rowNum(r, "donated")),
		MatchedUSD:       round2(rowNum(r, "matched")),
	}
}

func sortRoundsNewest(rs []GitcoinRound) {
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].LastDonation > rs[j].LastDonation })
}

func sortProjectsByFunding(ps []GitcoinProject) {
	sort.SliceStable(ps, func(i, j int) bool {
		return ps[i].DonatedUSD+ps[i].MatchedUSD > ps[j].DonatedUSD+ps[j].MatchedUSD
	})
}

func clampLimit(n, def int) int {
	if n <= 0 {
		return def
	}
	if n > 100 {
		return 100
	}
	return n
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
