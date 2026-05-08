package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type StringUint64 uint64

func (v StringUint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(v), 10))
}

func (v *StringUint64) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return fmt.Errorf("empty uint64 value")
	}

	var text string
	if raw[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		text = strings.TrimSpace(text)
	} else {
		if strings.ContainsAny(raw, ".eE+-") {
			return fmt.Errorf("invalid uint64 value %q", raw)
		}
		text = raw
	}

	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid uint64 value %q: %w", text, err)
	}
	*v = StringUint64(parsed)
	return nil
}

func (v StringUint64) Uint64() uint64 {
	return uint64(v)
}

func (v StringUint64) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func stringUint64SliceToUint64(values []StringUint64) []uint64 {
	result := make([]uint64, len(values))
	for i, value := range values {
		result[i] = value.Uint64()
	}
	return result
}
