package data

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// osoStub serves OSO's async SQL protocol: POST queues a job, the first status
// poll reports it running, the next reports it completed with a result URL.
// result returns the JSON-lines body for a submitted query.
func osoStub(t *testing.T, result func(sql string) string) (*OSOClient, *int32) {
	t.Helper()
	var posts int32
	var srv *httptest.Server
	var lastSQL string
	var polls int32
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/result":
			if r.Header.Get("Authorization") != "" {
				t.Error("result download must not carry the API key")
			}
			io.WriteString(w, result(lastSQL))
		case r.Method == http.MethodPost:
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("missing bearer auth, got %q", r.Header.Get("Authorization"))
			}
			atomic.AddInt32(&posts, 1)
			var body struct{ Query string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			lastSQL = body.Query
			atomic.StoreInt32(&polls, 0)
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"q1","status":"queued"}`)
		default:
			if r.URL.Query().Get("id") != "q1" {
				t.Errorf("status poll for unexpected id %q", r.URL.Query().Get("id"))
			}
			if atomic.AddInt32(&polls, 1) == 1 {
				io.WriteString(w, `{"id":"q1","status":"running"}`)
				return
			}
			io.WriteString(w, `{"id":"q1","status":"completed","url":"`+srv.URL+`/result"}`)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewOSOClient("test-key")
	c.url, c.poll = srv.URL+"/v1/async-sql", time.Millisecond
	return c, &posts
}

func TestOSOQueryRunsAsyncJobAndCaches(t *testing.T) {
	c, posts := osoStub(t, func(string) string {
		return "[\"name\",\"n\",\"x\"]\n[\"rotki\",3,null]\n\n[\"dappnode\",1.5,\"y\"]\n"
	})
	rows, err := c.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["name"] != "rotki" || rows[1]["n"] != 1.5 || rows[0]["x"] != nil {
		t.Fatalf("unexpected rows: %v", rows)
	}
	if _, err := c.Query(context.Background(), "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if *posts != 1 {
		t.Fatalf("repeat query should be served from cache, got %d submissions", *posts)
	}
}

func TestOSOQueryWithoutKey(t *testing.T) {
	_, err := NewOSOClient("").Query(context.Background(), "SELECT 1")
	if !errors.Is(err, ErrOSONoKey) {
		t.Fatalf("want ErrOSONoKey, got %v", err)
	}
}

func TestOSOQueryReportsFailedJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"id":"q9","status":"queued"}`)
			return
		}
		io.WriteString(w, `{"id":"q9","status":"failed"}`)
	}))
	defer srv.Close()
	c := NewOSOClient("k")
	c.url, c.poll = srv.URL, time.Millisecond
	if _, err := c.Query(context.Background(), "SELECT 1"); err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("want failed-job error, got %v", err)
	}
}

func TestSQLLiteralsRejectInjection(t *testing.T) {
	if got, err := sqlAddress("0xE2F7cF9C2b12c0BfcdAB571F9E50418fC08F4AD1"); err != nil || got != "'0xe2f7cf9c2b12c0bfcdab571f9e50418fc08f4ad1'" {
		t.Fatalf("address literal: %q %v", got, err)
	}
	for _, bad := range []string{"0x12", "0xe2f7cf9c2b12c0bfcdab571f9e50418fc08f4ad1' OR '1'='1", ""} {
		if _, err := sqlAddress(bad); err == nil {
			t.Errorf("address %q should be rejected", bad)
		}
	}
	if got, err := sqlName("O'Reilly Labs"); err != nil || got != "'o''reilly labs'" {
		t.Fatalf("name literal: %q %v", got, err)
	}
	for _, bad := range []string{"x; DROP TABLE t", "a\nb", strings.Repeat("a", 81), "%"} {
		if _, err := sqlName(bad); err == nil {
			t.Errorf("name %q should be rejected", bad)
		}
	}
	if got, err := sqlRoundFilter("42161:867"); err != nil || got != "chain_id = 42161 AND lower(round_id) = '867'" {
		t.Fatalf("round filter: %q %v", got, err)
	}
	if _, err := sqlRoundFilter("1:0x5aa255D5cAE9B6cE0F2d9aEE209cb02349b83731"); err != nil {
		t.Errorf("address-style round id rejected: %v", err)
	}
	for _, bad := range []string{"867", "1:1' OR 1=1", "x:867"} {
		if _, err := sqlRoundFilter(bad); err == nil {
			t.Errorf("round ref %q should be rejected", bad)
		}
	}
}

func TestSignalsFromMetrics(t *testing.T) {
	s := signalsFromMetrics([]map[string]any{
		{"metric_name": "GITHUB_stars_over_all_time", "amount": 3398.0, "sample_date": "2026-09-27"},
		{"metric_name": "GITHUB_contributors_over_all_time", "amount": 575.0, "sample_date": "2026-09-27"},
		{"metric_name": "GITCOIN_MATCHING_funding_awarded_over_all_time", "amount": 437212.3, "sample_date": "2026-09-26"},
		{"metric_name": "OCTANT_funding_awarded_over_all_time", "amount": 293658.0, "sample_date": "2026-09-27"},
		{"metric_name": "GITHUB_issue_age_avg_over_all_time", "amount": 1.0, "sample_date": "2026-09-27"},
	})
	if s.Code == nil || s.Code.Stars != 3398 || s.Code.Contributors != 575 {
		t.Fatalf("code metrics: %+v", s.Code)
	}
	if s.FundingUSD["GITCOIN_MATCHING"] != 437212.3 || s.FundingUSD["OCTANT"] != 293658 || len(s.FundingUSD) != 2 {
		t.Fatalf("funding: %v", s.FundingUSD)
	}
	if s.AsOf != "2026-09-27" {
		t.Fatalf("asOf: %q", s.AsOf)
	}
	if empty := signalsFromMetrics(nil); empty.Code != nil || empty.FormatSignals() != noOSOData {
		t.Fatalf("empty signals should format as no data: %+v", empty)
	}
}

func TestGitcoinFindByAddressFoldsRoundsAndTotal(t *testing.T) {
	c, _ := osoStub(t, func(sql string) string {
		if !strings.Contains(sql, "recipient_address = '0x9531c059098e3d194ff87febb587ab07b30b1306'") ||
			!strings.Contains(sql, "GROUPING SETS") {
			t.Errorf("unexpected query: %s", sql)
		}
		return `["round_id","round_name","round_number","chain","chain_id","donations","donors","donated","matched","name","oso","last_time","rounds"]
["r-old","GR15",15,"ethereum",1,100,90,500.004,1000,"rotki","rotki","2022-09-20 10:00:00",1]
[null,"GG23",23,"arbitrum",42161,150,120,700,1500,"rotki","rotki","2025-04-16 23:31:42",2]
["r-new","GG23",23,"arbitrum",42161,50,40,199.996,500,"rotki","rotki","2025-04-16 23:31:42",1]`
	})
	p, err := NewGitcoinClient(c).FindByAddress(context.Background(), "0x9531C059098e3d194fF87FebB587aB07B30B1306")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.RoundCount != 2 || p.UniqueDonors != 120 || p.DonatedUSD != 700 || p.OSOProject != "rotki" {
		t.Fatalf("total: %+v", p)
	}
	if len(p.Rounds) != 2 || p.Rounds[0].Ref != "42161:r-new" || p.Rounds[1].DonatedUSD != 500 {
		t.Fatalf("rounds should be newest first: %+v", p.Rounds)
	}
}

func TestGitcoinFindByAddressNotFound(t *testing.T) {
	c, _ := osoStub(t, func(string) string {
		return `["round_id","rounds"]
[null,0]`
	})
	p, err := NewGitcoinClient(c).FindByAddress(context.Background(), "0xe2F7cF9C2b12c0BfcdAB571F9E50418fC08F4AD1")
	if err != nil || p != nil {
		t.Fatalf("want nil, nil; got %+v, %v", p, err)
	}
}
