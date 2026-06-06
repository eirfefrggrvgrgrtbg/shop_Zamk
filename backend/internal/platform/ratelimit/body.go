package ratelimit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

const maxKeyBodyBytes = 64 * 1024

func JSONStringField(r *http.Request, field string) (string, bool) {
	if r.Body == nil {
		return "", false
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxKeyBodyBytes))
	if err != nil {
		return "", false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}

	value, ok := payload[field].(string)
	return value, ok
}
