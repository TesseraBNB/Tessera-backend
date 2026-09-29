package server

import (
	"net/http"
	"sort"

	"github.com/yeheskieltame/tessera/internal/analysis"
)

func (a *App) handleCurrentEpoch(w http.ResponseWriter, r *http.Request) {
	ep, err := a.octant.GetCurrentEpoch(r.Context())
	if err != nil {
		a.jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"currentEpoch": ep.CurrentEpoch})
}

func (a *App) handleProjects(w http.ResponseWriter, r *http.Request) {
	epoch := parseEpoch(r)
	if epoch == 0 {
		a.jsonError(w, "epoch query parameter is required", http.StatusBadRequest)
		return
	}
	projects, err := a.octant.GetProjects(r.Context(), epoch)
	if err != nil {
		a.jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"epoch": epoch, "projects": projects, "count": len(projects)})
}

func (a *App) handleAnalyzeEpoch(w http.ResponseWriter, r *http.Request) {
	epoch := parseEpoch(r)
	if epoch == 0 {
		a.jsonError(w, "epoch query parameter is required", http.StatusBadRequest)
		return
	}
	rewards, err := a.octant.GetProjectRewards(r.Context(), epoch)
	if err != nil {
		a.jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(rewards) == 0 {
		a.writeJSON(w, http.StatusOK, map[string]any{"epoch": epoch, "projects": []any{}})
		return
	}

	metrics := make([]analysis.ProjectMetrics, len(rewards))
	for i, rw := range rewards {
		al, mt := analysis.WeiToEth(rw.Allocated), analysis.WeiToEth(rw.Matched)
		metrics[i] = analysis.ProjectMetrics{Address: rw.Address, Allocated: al, Matched: mt, TotalFunding: al + mt}
	}
	metrics = analysis.ComputeCompositeScores(metrics)
	k := 4
	if len(metrics) < k {
		k = len(metrics)
	}
	metrics = analysis.SimpleKMeans(metrics, k)
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].CompositeScore > metrics[j].CompositeScore })

	type projectResult struct {
		Address string  `json:"address"`
		Alloc   float64 `json:"allocated"`
		Matched float64 `json:"matched"`
		Score   float64 `json:"score"`
		Cluster int     `json:"cluster"`
		Rank    int     `json:"rank"`
	}
	out := make([]projectResult, len(metrics))
	for i, m := range metrics {
		out[i] = projectResult{m.Address, m.Allocated, m.Matched, m.CompositeScore, m.Cluster, i + 1}
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"epoch": epoch, "projects": out})
}

func (a *App) handleDetectAnomalies(w http.ResponseWriter, r *http.Request) {
	epoch := parseEpoch(r)
	if epoch == 0 {
		a.jsonError(w, "epoch query parameter is required", http.StatusBadRequest)
		return
	}
	allocations, err := a.octant.GetAllocations(r.Context(), epoch)
	if err != nil {
		a.jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(allocations) == 0 {
		a.writeJSON(w, http.StatusOK, map[string]any{"epoch": epoch, "report": nil})
		return
	}

	donors := make([]string, len(allocations))
	amounts := make([]float64, len(allocations))
	for i, al := range allocations {
		donors[i] = al.Donor
		amounts[i] = analysis.WeiToEth(al.Amount)
	}
	report := analysis.DetectAnomalies(donors, amounts)
	a.writeJSON(w, http.StatusOK, map[string]any{
		"epoch": epoch,
		"report": map[string]any{
			"totalDonations":     report.TotalDonations,
			"uniqueDonors":       report.UniqueDonors,
			"totalAmount":        report.TotalAmount,
			"meanDonation":       report.MeanDonation,
			"medianDonation":     report.MedianDonation,
			"maxDonation":        report.MaxDonation,
			"whaleConcentration": report.WhaleConcentration,
			"flags":              report.Flags,
		},
	})
}

func (a *App) handleTrustGraph(w http.ResponseWriter, r *http.Request) {
	epoch := parseEpoch(r)
	if epoch == 0 {
		a.jsonError(w, "epoch query parameter is required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	allocations, err := a.octant.GetAllocations(ctx, epoch)
	if err != nil {
		a.jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(allocations) == 0 {
		a.writeJSON(w, http.StatusOK, map[string]any{"epoch": epoch, "profiles": []any{}})
		return
	}

	projects := make([]string, len(allocations))
	donors := make([]string, len(allocations))
	amounts := make([]float64, len(allocations))
	for i, al := range allocations {
		projects[i] = al.Project
		donors[i] = al.Donor
		amounts[i] = analysis.WeiToEth(al.Amount)
	}

	var prevDonors map[string]bool
	if epoch > 1 {
		if prev, err := a.octant.GetAllocations(ctx, epoch-1); err == nil {
			prevDonors = map[string]bool{}
			for _, al := range prev {
				prevDonors[al.Donor] = true
			}
		}
	}

	profiles := analysis.BuildTrustProfiles(projects, amounts, donors, prevDonors)

	type trustJSON struct {
		Address          string   `json:"address"`
		DonorCount       int      `json:"donorCount"`
		UniqueDonors     int      `json:"uniqueDonors"`
		DonorDiversity   float64  `json:"donorDiversity"`
		WhaleDepRatio    float64  `json:"whaleDepRatio"`
		CoordinationRisk float64  `json:"coordinationRisk"`
		RepeatDonors     int      `json:"repeatDonors"`
		Flags            []string `json:"flags"`
	}
	out := make([]trustJSON, len(profiles))
	for i, p := range profiles {
		flags := p.Flags
		if flags == nil {
			flags = []string{}
		}
		out[i] = trustJSON{p.Address, p.DonorCount, p.UniqueDonors, p.DonorDiversity, p.WhaleDepRatio, p.CoordinationRisk, p.RepeatDonors, flags}
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"epoch": epoch, "profiles": out})
}

func (a *App) handleSimulate(w http.ResponseWriter, r *http.Request) {
	epoch := parseEpoch(r)
	if epoch == 0 {
		a.jsonError(w, "epoch query parameter is required", http.StatusBadRequest)
		return
	}
	allocations, err := a.octant.GetAllocations(r.Context(), epoch)
	if err != nil {
		a.jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(allocations) == 0 {
		a.writeJSON(w, http.StatusOK, map[string]any{"epoch": epoch, "mechanisms": []any{}})
		return
	}

	projects := make([]string, len(allocations))
	donors := make([]string, len(allocations))
	amounts := make([]float64, len(allocations))
	inputs := make([]analysis.AllocationInput, len(allocations))
	for i, al := range allocations {
		eth := analysis.WeiToEth(al.Amount)
		projects[i] = al.Project
		donors[i] = al.Donor
		amounts[i] = eth
		inputs[i] = analysis.AllocationInput{Donor: al.Donor, Project: al.Project, Amount: eth}
	}

	trustProfiles := analysis.BuildTrustProfiles(projects, amounts, donors, nil)
	trustScores := map[string]float64{}
	for _, tp := range trustProfiles {
		trustScores[tp.Address] = tp.DonorDiversity
	}

	original := analysis.SimulateStandardQF(inputs)
	original.Name = "Original (Standard QF)"
	capped := analysis.SimulateCappedQF(inputs, 0.10)
	equal := analysis.SimulateEqualWeight(inputs)
	trustWeighted := analysis.SimulateTrustWeightedQF(inputs, trustScores)

	a.writeJSON(w, http.StatusOK, map[string]any{
		"epoch": epoch,
		"mechanisms": []map[string]any{
			marshalMechanism(original),
			marshalMechanism(capped),
			marshalMechanism(equal),
			marshalMechanism(trustWeighted),
		},
	})
}

func marshalMechanism(m analysis.MechanismResult) map[string]any {
	type simProj struct {
		Address       string  `json:"address"`
		Allocated     float64 `json:"allocated"`
		OriginalAlloc float64 `json:"originalAlloc"`
		Change        float64 `json:"change"`
	}
	projs := make([]simProj, len(m.Projects))
	for i, p := range m.Projects {
		projs[i] = simProj{p.Address, p.Allocated, p.OriginalAlloc, p.Change}
	}
	return map[string]any{
		"name":           m.Name,
		"description":    m.Description,
		"giniCoeff":      m.GiniCoeff,
		"topShare":       m.TopShare,
		"aboveThreshold": m.AboveThreshold,
		"projects":       projs,
	}
}
