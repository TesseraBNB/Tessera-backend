package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Open Source Observer serves its data lake over an async SQL API (Trino
// dialect): POST a query, poll until it completes, then download the result as
// JSON lines. It needs an API key from https://www.oso.xyz (organization
// Settings → API Keys). The older GraphQL API at www.opensource.observer is gone.
const (
	osoSQLURL   = "https://api.oso.xyz/v1/async-sql"
	osoTimeout  = 90 * time.Second
	osoCacheTTL = time.Hour // OSO refreshes daily; Gitcoin history is static
)

// ErrOSONoKey is returned by every OSO-backed call when no API key is configured.
var ErrOSONoKey = errors.New("OSO_API_KEY is not set: Open Source Observer and Gitcoin data are unavailable")

var (
	reEVMAddress = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	reSafeName   = regexp.MustCompile(`^[\p{L}\p{N} ._&'/-]{1,80}$`)
	reRoundRef   = regexp.MustCompile(`^(\d{1,12}):([0-9A-Za-z_-]{1,80})$`)
)

// OSOClient runs read-only SQL against OSO's public data lake.
type OSOClient struct {
	url    string
	apiKey string
	poll   time.Duration
	cache  *ttlCache
}

func NewOSOClient(apiKey string) *OSOClient {
	return &OSOClient{url: osoSQLURL, apiKey: apiKey, poll: time.Second, cache: newTTLCache(osoCacheTTL)}
}

// Enabled reports whether an API key is configured.
func (c *OSOClient) Enabled() bool { return c.apiKey != "" }

type osoJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// Query runs one SQL statement and returns its rows keyed by column name.
func (c *OSOClient) Query(ctx context.Context, sql string) ([]map[string]any, error) {
	if !c.Enabled() {
		return nil, ErrOSONoKey
	}
	if b, ok := c.cache.get(sql); ok {
		return parseJSONLines(b)
	}
	ctx, cancel := context.WithTimeout(ctx, osoTimeout)
	defer cancel()

	headers := map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + c.apiKey}
	body, _ := json.Marshal(map[string]string{"query": sql})
	raw, err := postJSON(ctx, c.url, body, headers)
	if err != nil {
		return nil, fmt.Errorf("OSO query: %w", err)
	}
	var job osoJob
	if err := json.Unmarshal(raw, &job); err != nil {
		return nil, fmt.Errorf("OSO query: unexpected response: %s", clip(raw, 200))
	}
	for job.Status != "completed" {
		if job.Status == "failed" || job.Status == "canceled" || job.ID == "" {
			return nil, fmt.Errorf("OSO query %s: %s", orDefault(job.Status, "rejected"), clip(raw, 200))
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("OSO query timed out: %w", ctx.Err())
		case <-time.After(c.poll):
		}
		if raw, err = getJSON(ctx, c.url+"?id="+url.QueryEscape(job.ID), headers); err != nil {
			return nil, fmt.Errorf("OSO query status: %w", err)
		}
		if err := json.Unmarshal(raw, &job); err != nil {
			return nil, fmt.Errorf("OSO query status: unexpected response: %s", clip(raw, 200))
		}
	}
	if job.URL == "" {
		return nil, errors.New("OSO query completed without a result URL")
	}
	// The result URL is pre-signed, so it must not carry the API key.
	res, err := getJSON(ctx, job.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("OSO query result: %w", err)
	}
	rows, err := parseJSONLines(res)
	if err == nil {
		c.cache.set(sql, res)
	}
	return rows, err
}

// parseJSONLines turns an OSO result file — a header row of column names, then
// one JSON array per row — into maps.
func parseJSONLines(raw []byte) ([]map[string]any, error) {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, nil
	}
	var cols []string
	if err := json.Unmarshal([]byte(lines[0]), &cols); err != nil {
		return nil, fmt.Errorf("OSO result header: %w", err)
	}
	rows := make([]map[string]any, 0, len(lines)-1)
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var vals []any
		if err := json.Unmarshal([]byte(l), &vals); err != nil {
			return nil, fmt.Errorf("OSO result row: %w", err)
		}
		r := make(map[string]any, len(cols))
		for i, col := range cols {
			if i < len(vals) {
				r[col] = vals[i]
			}
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// --- SQL literal helpers: every value that reaches a query is validated first ---

func sqlString(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func sqlAddress(addr string) (string, error) {
	if !reEVMAddress.MatchString(addr) {
		return "", fmt.Errorf("invalid address %q", addr)
	}
	return sqlString(strings.ToLower(addr)), nil
}

func sqlName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !reSafeName.MatchString(name) {
		return "", fmt.Errorf("invalid name %q (letters, digits, spaces and . _ & ' / - only, up to 80 characters)", name)
	}
	return sqlString(strings.ToLower(name)), nil
}

// sqlRoundFilter turns a round reference "chainId:roundId" into a WHERE clause.
// Gitcoin round ids are only unique within a chain.
func sqlRoundFilter(ref string) (string, error) {
	m := reRoundRef.FindStringSubmatch(strings.TrimSpace(ref))
	if m == nil {
		return "", fmt.Errorf("invalid round reference %q: use chainId:roundId, e.g. 42161:867", ref)
	}
	return "chain_id = " + m[1] + " AND lower(round_id) = " + sqlString(strings.ToLower(m[2])), nil
}

// --- typed accessors for result rows ---

func rowStr(r map[string]any, k string) string {
	switch v := r[k].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func rowNum(r map[string]any, k string) float64 {
	switch v := r[k].(type) {
	case float64:
		return v
	case string:
		var f float64
		_, _ = fmt.Sscan(v, &f)
		return f
	}
	return 0
}

func rowInt(r map[string]any, k string) int { return int(rowNum(r, k)) }

// --- projects and signals ---

// OSOProject is a project in OSO's curated directory (oss-directory).
type OSOProject struct {
	ID          string `json:"-"`
	Name        string `json:"name"` // OSO slug, e.g. "rotki"
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}

// CodeMetrics are all-time GitHub totals across a project's repositories.
type CodeMetrics struct {
	Stars              float64 `json:"stars"`
	Forks              float64 `json:"forks"`
	Contributors       float64 `json:"contributors"`
	Commits            float64 `json:"commits"`
	MergedPullRequests float64 `json:"mergedPullRequests"`
	Repositories       float64 `json:"repositories"`
	Releases           float64 `json:"releases"`
	OpenedIssues       float64 `json:"openedIssues"`
	ClosedIssues       float64 `json:"closedIssues"`
}

// ProjectSignals is what OSO knows about one project.
type ProjectSignals struct {
	Name        string             `json:"name"`
	DisplayName string             `json:"displayName,omitempty"`
	Found       bool               `json:"found"`
	MatchedBy   string             `json:"matchedBy,omitempty"` // "name" or "address"
	Code        *CodeMetrics       `json:"code,omitempty"`
	FundingUSD  map[string]float64 `json:"fundingUsdAllTime,omitempty"` // by source, e.g. GITCOIN_MATCHING, OCTANT
	Repos       []string           `json:"githubRepos,omitempty"`       // owner/repo, up to 10
	AsOf        string             `json:"asOf,omitempty"`
	Candidates  []OSOProject       `json:"candidates,omitempty"` // near matches when the name was not exact
	Note        string             `json:"note,omitempty"`
}

const osoProjects = "oso.projects_v1"

// FindProject looks a project up by OSO slug or display name. An exact match
// wins; otherwise up to five near matches are returned as candidates.
func (c *OSOClient) FindProject(ctx context.Context, name string) (*OSOProject, []OSOProject, error) {
	lit, err := sqlName(name)
	if err != nil {
		return nil, nil, err
	}
	q := `SELECT project_id, project_name, display_name, description FROM ` + osoProjects +
		` WHERE project_source = 'OSS_DIRECTORY' AND (strpos(lower(project_name), ` + lit + `) > 0 OR strpos(lower(display_name), ` + lit + `) > 0)` +
		` ORDER BY length(project_name) LIMIT 6`
	rows, err := c.Query(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	want := strings.ToLower(strings.TrimSpace(name))
	var near []OSOProject
	for _, r := range rows {
		p := OSOProject{ID: rowStr(r, "project_id"), Name: rowStr(r, "project_name"), DisplayName: rowStr(r, "display_name"), Description: rowStr(r, "description")}
		if strings.ToLower(p.Name) == want || strings.ToLower(p.DisplayName) == want {
			return &p, nil, nil
		}
		near = append(near, p)
	}
	return nil, near, nil
}

// FindProjectByAddress maps an on-chain address (payout wallet, contract) to
// the OSO project that lists it.
func (c *OSOClient) FindProjectByAddress(ctx context.Context, addr string) (*OSOProject, error) {
	lit, err := sqlAddress(addr)
	if err != nil {
		return nil, err
	}
	q := `SELECT DISTINCT p.project_id, p.project_name, p.display_name, p.description FROM oso.artifacts_by_project_v1 a` +
		` JOIN ` + osoProjects + ` p ON a.project_id = p.project_id` +
		` WHERE a.artifact_name = ` + lit + ` AND p.project_source = 'OSS_DIRECTORY' LIMIT 1`
	rows, err := c.Query(ctx, q)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	r := rows[0]
	return &OSOProject{ID: rowStr(r, "project_id"), Name: rowStr(r, "project_name"), DisplayName: rowStr(r, "display_name"), Description: rowStr(r, "description")}, nil
}

// Signals collects a project's all-time code metrics, funding by source and
// GitHub repositories.
func (c *OSOClient) Signals(ctx context.Context, p *OSOProject) (*ProjectSignals, error) {
	id := sqlString(p.ID)
	metricsQ := `SELECT m.metric_name, k.amount, CAST(k.sample_date AS varchar) AS sample_date FROM oso.key_metrics_by_project_v0 k` +
		` JOIN oso.metrics_v0 m ON k.metric_id = m.metric_id WHERE k.project_id = ` + id
	reposQ := `SELECT DISTINCT artifact_namespace, artifact_name FROM oso.artifacts_by_project_v1` +
		` WHERE project_id = ` + id + ` AND artifact_source = 'GITHUB' ORDER BY 1, 2 LIMIT 10`

	var (
		wg             sync.WaitGroup
		metrics, repos []map[string]any
		mErr, rErr     error
	)
	wg.Add(2)
	go func() { defer wg.Done(); metrics, mErr = c.Query(ctx, metricsQ) }()
	go func() { defer wg.Done(); repos, rErr = c.Query(ctx, reposQ) }()
	wg.Wait()
	if mErr != nil {
		return nil, mErr
	}

	s := signalsFromMetrics(metrics)
	s.Name, s.DisplayName, s.Found = p.Name, p.DisplayName, true
	if rErr == nil {
		for _, r := range repos {
			s.Repos = append(s.Repos, rowStr(r, "artifact_namespace")+"/"+rowStr(r, "artifact_name"))
		}
	}
	return s, nil
}

// signalsFromMetrics folds key_metrics_by_project rows (metric_name, amount)
// into code totals and funding by source.
func signalsFromMetrics(rows []map[string]any) *ProjectSignals {
	s := &ProjectSignals{}
	code := &CodeMetrics{}
	hasCode := false
	codeField := map[string]*float64{
		"GITHUB_stars_over_all_time":                &code.Stars,
		"GITHUB_forks_over_all_time":                &code.Forks,
		"GITHUB_contributors_over_all_time":         &code.Contributors,
		"GITHUB_commits_over_all_time":              &code.Commits,
		"GITHUB_merged_pull_requests_over_all_time": &code.MergedPullRequests,
		"GITHUB_repositories_over_all_time":         &code.Repositories,
		"GITHUB_releases_over_all_time":             &code.Releases,
		"GITHUB_opened_issues_over_all_time":        &code.OpenedIssues,
		"GITHUB_closed_issues_over_all_time":        &code.ClosedIssues,
	}
	for _, r := range rows {
		name, amt := rowStr(r, "metric_name"), rowNum(r, "amount")
		if d := rowStr(r, "sample_date"); d > s.AsOf {
			s.AsOf = d
		}
		if f, ok := codeField[name]; ok {
			*f, hasCode = amt, true
			continue
		}
		if src, ok := strings.CutSuffix(name, "_funding_awarded_over_all_time"); ok {
			if s.FundingUSD == nil {
				s.FundingUSD = map[string]float64{}
			}
			s.FundingUSD[src] = amt
		}
	}
	if hasCode {
		s.Code = code
	}
	return s
}

// CollectProjectSignals resolves a project by name and collects its signals.
// It never fails: problems are reported in Note so callers can show them.
func (c *OSOClient) CollectProjectSignals(ctx context.Context, name string) *ProjectSignals {
	p, near, err := c.FindProject(ctx, name)
	if err != nil {
		return &ProjectSignals{Name: name, Note: err.Error()}
	}
	if p == nil {
		return &ProjectSignals{Name: name, Candidates: near, Note: "no exact OSO project match"}
	}
	s, err := c.Signals(ctx, p)
	if err != nil {
		return &ProjectSignals{Name: p.Name, DisplayName: p.DisplayName, Found: true, Note: err.Error()}
	}
	s.MatchedBy = "name"
	return s
}

const noOSOData = "No OSO data available for this project."

// FormatSignals returns a human-readable summary of the collected signals.
func (s *ProjectSignals) FormatSignals() string {
	if s.Code == nil && len(s.FundingUSD) == 0 {
		return noOSOData
	}
	var b strings.Builder
	title := s.Name
	if s.DisplayName != "" && !strings.EqualFold(s.DisplayName, s.Name) {
		title = fmt.Sprintf("%s (%s)", s.DisplayName, s.Name)
	}
	fmt.Fprintf(&b, "### Open Source Observer — %s (as of %s)\n", title, orDefault(s.AsOf, "latest"))
	if c := s.Code; c != nil {
		fmt.Fprintf(&b, "- GitHub (all time): %.0f stars · %.0f forks · %.0f repos · %.0f contributors\n", c.Stars, c.Forks, c.Repositories, c.Contributors)
		fmt.Fprintf(&b, "- %.0f commits · %.0f merged PRs · %.0f releases · issues %.0f opened / %.0f closed\n", c.Commits, c.MergedPullRequests, c.Releases, c.OpenedIssues, c.ClosedIssues)
	}
	if len(s.FundingUSD) > 0 {
		srcs := make([]string, 0, len(s.FundingUSD))
		for k := range s.FundingUSD {
			srcs = append(srcs, k)
		}
		sort.Slice(srcs, func(i, j int) bool { return s.FundingUSD[srcs[i]] > s.FundingUSD[srcs[j]] })
		b.WriteString("- Funding received (all time, USD):")
		for _, k := range srcs {
			fmt.Fprintf(&b, " %s $%.0f;", k, s.FundingUSD[k])
		}
		b.WriteString("\n")
	}
	if len(s.Repos) > 0 {
		fmt.Fprintf(&b, "- Repositories: %s\n", strings.Join(s.Repos, ", "))
	}
	return b.String()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
