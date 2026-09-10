package httputil

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// StrictUnmarshal rejects unknown JSON fields to prevent client-side parameter injection.
func StrictUnmarshal(data []byte, dest any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected trailing json data")
	}
	return nil
}
