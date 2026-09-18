package logic

import (
	"database/sql"
	"testing"
	"time"
)

func TestNullTimeToStr(t *testing.T) {
	if got := nullTimeToStr(sql.NullTime{}); got != "" {
		t.Fatalf("nullTimeToStr(invalid)=%q want empty", got)
	}
	ts := time.Date(2026, 9, 18, 10, 30, 0, 0, time.UTC)
	if got := nullTimeToStr(sql.NullTime{Time: ts, Valid: true}); got != "2026-09-18 10:30:00" {
		t.Fatalf("nullTimeToStr(valid)=%q want 2026-09-18 10:30:00", got)
	}
}
