package upload

import (
	"strings"
	"testing"
	"time"

	"selfstudio/agent/internal/cloud"
	"selfstudio/agent/internal/sessions"
)

func TestBuildSessionObjectPrefixDeterministicAndSanitized(t *testing.T) {
	s := sessions.Session{SessionID: "Session 01", StationID: "station/A", CustomerName: " ACME../ Bride ", OrderNumber: "ORD #42", StartedAt: time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)}
	cfg := cloud.Settings{Provider: cloud.ProviderGCS, BucketName: "selfstudio-bucket", TargetRootPrefix: "events/2026"}
	prefix1, identity1, err := BuildSessionObjectPrefix(cfg, s)
	if err != nil {
		t.Fatalf("prefix: %v", err)
	}
	prefix2, identity2, err := BuildSessionObjectPrefix(cfg, s)
	if err != nil {
		t.Fatalf("prefix retry: %v", err)
	}
	if prefix1 != prefix2 || identity1 != identity2 {
		t.Fatalf("not deterministic: %q/%q vs %q/%q", prefix1, identity1, prefix2, identity2)
	}
	want := "events/2026/2026/05/19/acme-bride/ord-42/stationa/session-01"
	if prefix1 != want {
		t.Fatalf("prefix = %q want %q", prefix1, want)
	}
	if strings.Contains(prefix1, "..") || strings.HasPrefix(prefix1, "/") || strings.Contains(prefix1, "\\") {
		t.Fatalf("unsafe prefix: %q", prefix1)
	}
}

func TestValidateObjectPrefixRejectsUnsafe(t *testing.T) {
	for _, prefix := range []string{"", "/root", "root/../x", "root/./x", "root\\x", strings.Repeat("a", 901)} {
		if err := ValidateObjectPrefix(prefix); err == nil {
			t.Fatalf("expected reject for %q", prefix)
		}
	}
}
