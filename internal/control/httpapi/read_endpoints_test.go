package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseTimeRangeUsesProvidedToAsDefaultAnchor(t *testing.T) {
	now := time.Date(2026, time.March, 15, 12, 0, 0, 0, time.UTC)
	to := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)

	req := httptest.NewRequest("GET", "/v1/control/orgs/org_1/usage/summary?to="+to.Format(time.RFC3339), nil)
	rec := httptest.NewRecorder()

	from, gotTo, ok := parseTimeRange(rec, req, now)
	if !ok {
		t.Fatalf("expected parseTimeRange to succeed, got response %q", rec.Body.String())
	}

	if !gotTo.Equal(to) {
		t.Fatalf("expected to=%s, got %s", to.Format(time.RFC3339), gotTo.Format(time.RFC3339))
	}
	expectedFrom := to.Add(-30 * 24 * time.Hour)
	if !from.Equal(expectedFrom) {
		t.Fatalf("expected from=%s, got %s", expectedFrom.Format(time.RFC3339), from.Format(time.RFC3339))
	}
}
