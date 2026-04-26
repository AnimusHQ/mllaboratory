package registryverify

import "encoding/json"

func SanitizeDetails(details map[string]any) json.RawMessage {
	if len(details) == 0 {
		return json.RawMessage(`{}`)
	}
	allowed := map[string]struct{}{
		"reason":      {},
		"error":       {},
		"status_code": {},
	}
	out := map[string]any{}
	for key, value := range details {
		if _, ok := allowed[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return json.RawMessage(`{}`)
	}
	blob, err := json.Marshal(out)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return blob
}
