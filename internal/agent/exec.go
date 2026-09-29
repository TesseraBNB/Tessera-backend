package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yeheskieltame/tessera/internal/analysis"
	"github.com/yeheskieltame/tessera/internal/data"
)

// toolDef pairs a tool's advertised schema with its in-process executor.
type toolDef struct {
	tool Tool
	exec func(ctx context.Context, input json.RawMessage) (string, error)
}

// buildRegistry wires every tool the agent can call to a local executor backed
// by Tessera's data and analysis layers. Executors run in-process, so no tool
// endpoint is ever exposed to the network.
func (c *Client) buildRegistry() map[string]toolDef {
	defs := []toolDef{
		{
			tool: Tool{Name: "get_current_epoch", Description: "Get Octant's current funding epoch number.", InputSchema: schemaEmpty},
			exec: func(ctx context.Context, _ json.RawMessage) (string, error) {
				ep, err := c.octant.GetCurrentEpoch(ctx)
				if err != nil {
					return "", err
				}
				return jsonStr(map[string]int{"currentEpoch": ep.CurrentEpoch})
			},
		},
		{
			tool: Tool{Name: "get_project_history", Description: "Octant funding history (allocated/matched ETH and donor count per epoch) for a project address across all epochs.", InputSchema: schemaAddress},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				var a struct {
					Address string `json:"address"`
				}
				if err := json.Unmarshal(in, &a); err != nil {
					return "", err
				}
				if a.Address == "" {
					return "", fmt.Errorf("address is required")
				}
				ep, err := c.octant.GetCurrentEpoch(ctx)
				if err != nil {
					return "", err
				}
				hist, err := c.octant.GetProjectHistory(ctx, a.Address, 1, ep.CurrentEpoch)
				if err != nil {
					return "", err
				}
				if len(hist) == 0 {
					return "", fmt.Errorf("project %s not found in any Octant epoch", a.Address)
				}
				return jsonStr(hist)
			},
		},
		{
			tool: Tool{Name: "rank_projects", Description: "Rank all Octant projects in an epoch by composite funding score (0-100, weighted 40% allocated + 60% matched). Returns the ranked list with allocated/matched ETH.", InputSchema: schemaEpoch},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				epoch, err := c.epochArg(ctx, in)
				if err != nil {
					return "", err
				}
				rewards, err := c.octant.GetProjectRewards(ctx, epoch)
				if err != nil {
					return "", err
				}
				metrics := make([]analysis.ProjectMetrics, len(rewards))
				for i, rw := range rewards {
					al, mt := analysis.WeiToEth(rw.Allocated), analysis.WeiToEth(rw.Matched)
					metrics[i] = analysis.ProjectMetrics{Address: rw.Address, Allocated: al, Matched: mt, TotalFunding: al + mt}
				}
				metrics = analysis.ComputeCompositeScores(metrics)
				sort.Slice(metrics, func(i, j int) bool { return metrics[i].CompositeScore > metrics[j].CompositeScore })
				return jsonStr(map[string]any{"epoch": epoch, "count": len(metrics), "projects": metrics})
			},
		},
		{
			tool: Tool{Name: "get_trust_profile", Description: "Trust-graph metrics for an epoch: donor diversity (Shannon entropy), whale dependency, coordination/Sybil risk (max Jaccard donor overlap), and repeat donors. Provide an address for one project, or omit it for all projects.", InputSchema: schemaEpochAddrOptional},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				var args struct {
					Epoch   int    `json:"epoch"`
					Address string `json:"address"`
				}
				_ = json.Unmarshal(in, &args)
				epoch := args.Epoch
				if epoch == 0 {
					e, err := c.octant.GetCurrentEpoch(ctx)
					if err != nil {
						return "", err
					}
					epoch = e.CurrentEpoch
				}
				ad, err := c.allocData(ctx, epoch)
				if err != nil {
					return "", err
				}
				profiles := analysis.BuildTrustProfiles(ad.projects, ad.amounts, ad.donors, ad.prevDonors)
				if args.Address != "" {
					for _, p := range profiles {
						if strings.EqualFold(p.Address, args.Address) {
							return jsonStr(p)
						}
					}
					return "", fmt.Errorf("project %s not found in epoch %d allocations", args.Address, epoch)
				}
				return jsonStr(map[string]any{"epoch": epoch, "profiles": profiles})
			},
		},
		{
			tool: Tool{Name: "simulate_mechanisms", Description: "Simulate four funding mechanisms for an epoch (Standard QF, Capped QF at 10%, Equal Weight, Trust-Weighted QF) and compare the resulting distributions (Gini coefficient, top-project share). Reveals how robust the current allocation is to the funding rule.", InputSchema: schemaEpoch},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				epoch, err := c.epochArg(ctx, in)
				if err != nil {
					return "", err
				}
				ad, err := c.allocData(ctx, epoch)
				if err != nil {
					return "", err
				}
				inputs := make([]analysis.AllocationInput, len(ad.allocations))
				for i, a := range ad.allocations {
					inputs[i] = analysis.AllocationInput{Donor: a.Donor, Project: a.Project, Amount: analysis.WeiToEth(a.Amount)}
				}
				profiles := analysis.BuildTrustProfiles(ad.projects, ad.amounts, ad.donors, ad.prevDonors)
				trust := make(map[string]float64, len(profiles))
				for _, p := range profiles {
					trust[p.Address] = p.DonorDiversity
				}
				results := []analysis.MechanismResult{
					analysis.SimulateStandardQF(inputs),
					analysis.SimulateCappedQF(inputs, 0.10),
					analysis.SimulateEqualWeight(inputs),
					analysis.SimulateTrustWeightedQF(inputs, trust),
				}
				return jsonStr(map[string]any{"epoch": epoch, "mechanisms": results})
			},
		},
		{
			tool: Tool{Name: "scan_chain", Description: "Scan an EVM address across 11 chains (BNB Smart Chain, opBNB, Ethereum, Base, Optimism, Arbitrum, Mantle, Scroll, Linea, zkSync, BSC Testnet): native balance, tx count, contract status, and ERC-20 (USDC/USDT/DAI/FDUSD) balances.", InputSchema: schemaAddress},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				var a struct {
					Address string `json:"address"`
				}
				if err := json.Unmarshal(in, &a); err != nil {
					return "", err
				}
				if a.Address == "" {
					return "", fmt.Errorf("address is required")
				}
				return jsonStr(c.blockchain.ScanAddress(ctx, a.Address))
			},
		},
		{
			tool: Tool{Name: "get_oso_metrics", Description: "Open Source Observer signals for a project (GitHub code metrics, on-chain activity, funding history) by OSO project name.", InputSchema: schemaProjectName},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				name, err := stringArg(in, "project_name")
				if err != nil {
					return "", err
				}
				return jsonStr(c.oso.CollectProjectSignals(ctx, name))
			},
		},
		{
			tool: Tool{Name: "get_github_signals", Description: "GitHub repository signals (stars, forks, contributors, commit recency, README) to ground a project's codebase health.", InputSchema: schemaOwnerRepo},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				var a struct {
					Owner string `json:"owner"`
					Repo  string `json:"repo"`
				}
				if err := json.Unmarshal(in, &a); err != nil {
					return "", err
				}
				if a.Owner == "" || a.Repo == "" {
					return "", fmt.Errorf("owner and repo are required")
				}
				return jsonStr(c.github.CollectEvalSignals(ctx, a.Owner, a.Repo))
			},
		},
		{
			tool: Tool{Name: "get_forum_sentiment", Description: "Octant Discourse community signals for a project: thread count, engagement, likes, and team responsiveness.", InputSchema: schemaProjectName},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				name, err := stringArg(in, "project_name")
				if err != nil {
					return "", err
				}
				return jsonStr(c.discourse.CollectCommunitySignals(ctx, name))
			},
		},
		{
			tool: Tool{Name: "find_in_retropgf", Description: "Check whether a project appears in Optimism RetroPGF (cross-ecosystem validation): impact categories and funding received.", InputSchema: schemaRetro},
			exec: func(ctx context.Context, in json.RawMessage) (string, error) {
				var a struct {
					Name      string `json:"name"`
					Address   string `json:"address"`
					GithubURL string `json:"github_url"`
				}
				if err := json.Unmarshal(in, &a); err != nil {
					return "", err
				}
				if a.Name == "" {
					return "", fmt.Errorf("name is required")
				}
				return jsonStr(c.retropgf.FindInRetroPGF(ctx, a.Name, a.Address, a.GithubURL))
			},
		},
	}

	m := make(map[string]toolDef, len(defs))
	for _, d := range defs {
		m[d.tool.Name] = d
	}
	return m
}

// toolList returns the advertised tool schemas, sorted for deterministic output.
func (c *Client) toolList() []Tool {
	out := make([]Tool, 0, len(c.reg))
	for _, d := range c.reg {
		out = append(out, d.tool)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// execTool dispatches a tool_use request to its executor.
func (c *Client) execTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	d, ok := c.reg[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return d.exec(ctx, input)
}

// --- shared executor helpers ---

// allocBundle holds an epoch's allocations pre-shaped for the analysis package.
type allocBundle struct {
	allocations []data.Allocation
	projects    []string
	donors      []string
	amounts     []float64
	prevDonors  map[string]bool
}

func (c *Client) allocData(ctx context.Context, epoch int) (*allocBundle, error) {
	allocs, err := c.octant.GetAllocations(ctx, epoch)
	if err != nil {
		return nil, err
	}
	b := &allocBundle{
		allocations: allocs,
		projects:    make([]string, len(allocs)),
		donors:      make([]string, len(allocs)),
		amounts:     make([]float64, len(allocs)),
	}
	for i, a := range allocs {
		b.projects[i] = a.Project
		b.donors[i] = a.Donor
		b.amounts[i] = analysis.WeiToEth(a.Amount)
	}
	if epoch > 1 {
		if prev, err := c.octant.GetAllocations(ctx, epoch-1); err == nil {
			b.prevDonors = map[string]bool{}
			for _, a := range prev {
				b.prevDonors[a.Donor] = true
			}
		}
	}
	return b, nil
}

func (c *Client) epochArg(ctx context.Context, in json.RawMessage) (int, error) {
	var a struct {
		Epoch int `json:"epoch"`
	}
	_ = json.Unmarshal(in, &a)
	if a.Epoch > 0 {
		return a.Epoch, nil
	}
	ep, err := c.octant.GetCurrentEpoch(ctx)
	if err != nil {
		return 0, err
	}
	return ep.CurrentEpoch, nil
}

func stringArg(in json.RawMessage, key string) (string, error) {
	m := map[string]string{}
	if err := json.Unmarshal(in, &m); err != nil {
		return "", err
	}
	v := strings.TrimSpace(m[key])
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

func jsonStr(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// --- tool input JSON schemas ---

var (
	schemaEmpty             = json.RawMessage(`{"type":"object","properties":{}}`)
	schemaAddress           = json.RawMessage(`{"type":"object","properties":{"address":{"type":"string","description":"EVM address, 0x-prefixed"}},"required":["address"]}`)
	schemaEpoch             = json.RawMessage(`{"type":"object","properties":{"epoch":{"type":"integer","description":"Octant epoch number; omit for the latest epoch"}}}`)
	schemaEpochAddrOptional = json.RawMessage(`{"type":"object","properties":{"epoch":{"type":"integer","description":"Octant epoch number; omit for latest"},"address":{"type":"string","description":"Project address; omit to return all projects"}}}`)
	schemaProjectName       = json.RawMessage(`{"type":"object","properties":{"project_name":{"type":"string","description":"Project name"}},"required":["project_name"]}`)
	schemaOwnerRepo         = json.RawMessage(`{"type":"object","properties":{"owner":{"type":"string"},"repo":{"type":"string"}},"required":["owner","repo"]}`)
	schemaRetro             = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"address":{"type":"string"},"github_url":{"type":"string"}},"required":["name"]}`)
)
