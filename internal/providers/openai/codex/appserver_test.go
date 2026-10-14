package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseQuotaIncludesEveryRecognizedWindow(t *testing.T) {
	raw := []byte(`{
		"rateLimits": {
			"secondary": {"usedPercent": 21, "windowDurationMins": 10080, "resetsAt": 1786934800},
			"primary": {"usedPercent": 37, "windowDurationMins": 300, "resetsAt": 1786337200},
			"burst": {"usedPercent": 9, "windowDurationMins": 60, "resetsAt": 1786330000}
		},
		"rateLimitResetCredits": {"availableCount": 2},
		"account": {"email": "secret@example.com"}
	}`)
	quota, err := ParseQuota(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(quota.Windows) != 3 || quota.Windows[0].Name != "5 HOUR" || quota.ResetCredits != 2 {
		t.Fatalf("unexpected quota: %#v", quota)
	}
}

func TestParseQuotaRejectsImpossiblePercentage(t *testing.T) {
	_, err := ParseQuota([]byte(`{"rateLimits":{"primary":{"usedPercent":101,"windowDurationMins":300,"resetsAt":1}}}`))
	if err == nil {
		t.Fatal("invalid percentage accepted")
	}
}

func TestClientUsesOneInitializedSession(t *testing.T) {
	directory := t.TempDir()
	requests := filepath.Join(directory, "requests")
	server := filepath.Join(directory, "fake-codex")
	script := `#!/bin/sh
while IFS= read -r line; do
  printf '%s\n' "$line" >> "` + requests + `"
  case "$line" in
    *'"method":"initialize"'*) printf '%s\n' '{"id":1,"result":{"userAgent":"fake"}}' ;;
    *'"method":"account/rateLimits/read"'*) printf '%s\n' '{"id":2,"result":{"rateLimits":{"primary":{"usedPercent":10,"windowDurationMins":300,"resetsAt":1786337200}}}}' ;;
    *'"method":"thread/list"'*) printf '%s\n' '{"id":3,"result":{"data":[{"id":"thread-1","sessionId":"session-1","name":"Visible title","parentThreadId":null,"source":"appServer"}],"nextCursor":null}}' ;;
  esac
done
`
	if err := os.WriteFile(server, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	client := NewClient(server)
	defer client.Close()
	if _, err := client.ReadQuota(context.Background()); err != nil {
		t.Fatal(err)
	}
	threads, err := client.ReadThreads(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 || threads[0].Title != "Visible title" {
		t.Fatalf("unexpected threads: %#v", threads)
	}
	contents, _ := os.ReadFile(requests)
	if count := strings.Count(string(contents), `"method":"initialize"`); count != 1 {
		t.Fatalf("initialize count = %d, want 1", count)
	}
}

func TestRPCErrorDoesNotExposePayload(t *testing.T) {
	directory := t.TempDir()
	server := filepath.Join(directory, "fake-codex")
	script := `#!/bin/sh
read -r line
printf '%s\n' '{"id":1,"error":{"message":"SECRET_MARKER"}}'
`
	if err := os.WriteFile(server, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := NewClient(server).ReadQuota(context.Background())
	if err == nil || strings.Contains(err.Error(), "SECRET_MARKER") {
		t.Fatalf("unsafe error: %v", err)
	}
}
