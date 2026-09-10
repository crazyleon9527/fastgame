package handler

import (
	"context"
	"encoding/json"
	"fmt"
)

func userIDFromCtx(ctx context.Context) (uint64, error) {
	v := ctx.Value("userId")
	switch id := v.(type) {
	case float64:
		return uint64(id), nil
	case int64:
		return uint64(id), nil
	case json.Number:
		n, err := id.Int64()
		return uint64(n), err
	default:
		return 0, fmt.Errorf("missing user id in token")
	}
}
