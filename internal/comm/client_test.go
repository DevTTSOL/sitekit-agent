package comm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientDoesNotFollowRedirectOrForwardToken(t *testing.T) {
	redirectTargetHit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHit = true
		if r.Header.Get("Authorization") != "" {
			t.Error("agent token was forwarded to redirect target")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobs":[],"count":0}`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	client := NewClient(redirector.URL, "secret-token")
	_, err := client.FetchJobs(context.Background())
	if err == nil {
		t.Fatal("redirecting server response unexpectedly succeeded")
	}
	if redirectTargetHit {
		t.Fatal("client followed a redirect")
	}
}
