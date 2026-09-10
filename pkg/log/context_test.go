package log

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"fastgame/pkg/trace"

	"github.com/zeromicro/go-zero/core/logx"
)

func TestEnsureTraceAddsTraceField(t *testing.T) {
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	t.Cleanup(func() { logx.Reset() })

	ctx := trace.WithID(context.Background(), "trace-abc")
	C(ctx).Infow("test_event", logx.Field(KeyRoundID, "round-1"))

	out := buf.String()
	if !strings.Contains(out, "trace-abc") {
		t.Fatalf("expected trace_id in log output: %s", out)
	}
}

func TestWithRound(t *testing.T) {
	ctx := WithRound(context.Background(), "r-99")
	ctx = EnsureTrace(ctx)
	if ctx == nil {
		t.Fatal("nil ctx")
	}
}
