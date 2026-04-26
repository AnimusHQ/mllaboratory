package openapi3

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader provides minimal OpenAPI loading for offline linting.
type Loader struct {
	IsExternalRefsAllowed bool
}

// NewLoader returns a new Loader.
func NewLoader() *Loader {
	return &Loader{}
}

// T holds the parsed OpenAPI document.
type T struct {
	OpenAPI string
	Info    *Info
	Paths   map[string]any
	raw     map[string]any
}

// Info holds minimal info metadata.
type Info struct {
	Title   string
	Version string
}

// LoadFromFile reads and parses an OpenAPI document.
func (l *Loader) LoadFromFile(path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	docMap, ok := normalize(raw).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("openapi document is not a map")
	}
	doc := &T{raw: docMap}
	if value, ok := docMap["openapi"].(string); ok {
		doc.OpenAPI = value
	}
	if infoMap, ok := docMap["info"].(map[string]any); ok {
		doc.Info = &Info{
			Title:   stringify(infoMap["title"]),
			Version: stringify(infoMap["version"]),
		}
	}
	if paths, ok := docMap["paths"].(map[string]any); ok {
		doc.Paths = paths
	}
	return doc, nil
}

// Validate performs minimal structural validation.
func (t *T) Validate(_ any) error {
	if t == nil {
		return fmt.Errorf("openapi document is nil")
	}
	if strings.TrimSpace(t.OpenAPI) == "" {
		return fmt.Errorf("openapi field is required")
	}
	if t.Info == nil {
		return fmt.Errorf("info section is required")
	}
	if strings.TrimSpace(t.Info.Title) == "" {
		return fmt.Errorf("info.title is required")
	}
	if strings.TrimSpace(t.Info.Version) == "" {
		return fmt.Errorf("info.version is required")
	}
	if t.Paths == nil {
		return fmt.Errorf("paths section is required")
	}
	return nil
}

func normalize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[key] = normalize(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[fmt.Sprintf("%v", key)] = normalize(val)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalize(item)
		}
		return out
	default:
		return value
	}
}

func stringify(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
