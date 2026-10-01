package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// octantStub serves /epochs/current and per-epoch rewards from fixed bodies.
func octantStub(t *testing.T, current string, rewards map[string]string) *OctantClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/epochs/current" {
			_, _ = w.Write([]byte(current))
			return
		}
		if body, ok := rewards[r.URL.Path]; ok {
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(`{"rewards":[]}`))
	}))
	t.Cleanup(srv.Close)
	return &OctantClient{baseURL: srv.URL, cache: newTTLCache(time.Minute)}
}

func TestGetLatestFundedEpochSkipsEmptyAndSingleProjectEpochs(t *testing.T) {
	c := octantStub(t, `{"currentEpoch":5}`, map[string]string{
		"/rewards/projects/epoch/4": `{"rewards":[{"address":"0xa","allocated":"1","matched":"1"}]}`,
		"/rewards/projects/epoch/3": `{"rewards":[{"address":"0xa","allocated":"1","matched":"1"},{"address":"0xb","allocated":"1","matched":"1"}]}`,
	})

	got, err := c.GetLatestFundedEpoch(context.Background())
	if err != nil {
		t.Fatalf("GetLatestFundedEpoch: %v", err)
	}
	if got != 3 {
		t.Errorf("latest funded epoch = %d, want 3", got)
	}
}

func TestGetLatestFundedEpochErrorsWhenNoneFunded(t *testing.T) {
	c := octantStub(t, `{"currentEpoch":2}`, nil)
	if _, err := c.GetLatestFundedEpoch(context.Background()); err == nil {
		t.Error("expected error when no epoch has funded projects")
	}
}
