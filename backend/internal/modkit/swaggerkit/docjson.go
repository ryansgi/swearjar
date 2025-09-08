//go:build swag

package swaggerkit

import (
	"encoding/json"
	"net/http"
	"strings"

	"swearjar/internal/platform/config"

	docs "swearjar/internal/services/api/docs"
)

// SpecMutator lets modules tweak the parsed swagger spec before it is served
type SpecMutator func(map[string]any)

// mutators is the in process registry for spec mutators
var mutators []SpecMutator

// docReader is a seam so tests can inject invalid JSON without patching swagger
var docReader = func() string { return docs.SwaggerInfo.ReadDoc() }

// Register adds a spec mutator for swagger JSON
// call this from module init so it is wired automatically
func Register(m SpecMutator) {
	if m != nil {
		mutators = append(mutators, m)
	}
}

// serveDocJSON serves swagger JSON and lets modules adjust details
func serveDocJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := docReader()

		var spec map[string]any
		if err := json.Unmarshal([]byte(raw), &spec); err != nil {
			http.Error(w, "spec parse error", http.StatusInternalServerError)
			return
		}

		// OAS3 base url lives in servers, not BasePath
		ensureServers(spec, "/api/v1")

		// optional global tweaks go here
		cfg := config.New().Prefix("CORE_API_")
		if v := cfg.MayString("DOCS_TITLE_SUFFIX", ""); v != "" {
			if info, ok := spec["info"].(map[string]any); ok {
				if title, ok := info["title"].(string); ok {
					info["title"] = title + " " + v
				}
			}
		}

		ensureErrorResponseDefinition(spec)
		addDefaultError(spec)
		addDefaultBadRequest(spec)

		for _, m := range mutators {
			m(spec)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(spec)
	}
}

// ensureServers makes sure the spec is OAS3 and has a servers array
// swagger http ui can't support 3.1 at the moment, so downconvert if needed
func ensureServers(spec map[string]any, url string) {
	// if it's swagger 2, lift to oas3
	if _, hasSwagger := spec["swagger"]; hasSwagger {
		spec["openapi"] = "3.0.3"
		delete(spec, "swagger")
	}

	// if it's already oas3, downsample 3.1 -> 3.0.3
	if v, ok := spec["openapi"].(string); ok {
		if strings.HasPrefix(v, "3.1") {
			spec["openapi"] = "3.0.3"
		}
	} else {
		// no version set at all: pick a sane default
		spec["openapi"] = "3.0.3"
	}

	// ensure servers
	if _, ok := spec["servers"]; !ok {
		spec["servers"] = []any{
			map[string]any{"url": url},
		}
	}
}

// ensureErrorResponseDefinition creates a simple error envelope model if missing
// kept minimal so it does not drift from the runtime wire
func ensureErrorResponseDefinition(spec map[string]any) {
	comps, ok := spec["components"].(map[string]any)
	if !ok {
		comps = map[string]any{}
		spec["components"] = comps
	}
	schemas, ok := comps["schemas"].(map[string]any)
	if !ok {
		schemas = map[string]any{}
		comps["schemas"] = schemas
	}
	if _, ok := schemas["ErrorResponse"]; ok {
		return
	}
	schemas["ErrorResponse"] = map[string]any{
		"type":        "object",
		"description": "Standard error response",
		"properties": map[string]any{
			"status_code": map[string]any{"type": "integer", "format": "int32"},
			"status":      map[string]any{"type": "string"},
			"code":        map[string]any{"type": "integer", "format": "int32"},
			"error":       map[string]any{"type": "string"},
			"request_id":  map[string]any{"type": "string"},
		},
		"required": []any{"status_code", "status"},
	}
}

// addDefaultError walks every operation and injects a 500 response if absent
// OAS3 version using content.application/json.schema
func addDefaultError(spec map[string]any) {
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return
	}
	errResp := map[string]any{
		"description": "Internal Server Error",
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"},
				"example": map[string]any{
					"status_code": 500,
					"status":      "Internal Server Error",
					"code":        1,
					"error":       "panic recovered",
					"request_id":  "579f33bf50b1/abc-000001",
				},
			},
		},
	}
	for _, p := range paths {
		node, ok := p.(map[string]any)
		if !ok {
			continue
		}
		for _, opAny := range node {
			op, ok := opAny.(map[string]any)
			if !ok {
				continue
			}
			responses, ok := op["responses"].(map[string]any)
			if !ok {
				responses = map[string]any{}
				op["responses"] = responses
			}
			if _, exists := responses["500"]; !exists {
				responses["500"] = errResp
			}
		}
	}
}

// 400 with a 400-ish example (matches your binder output)
func addDefaultBadRequest(spec map[string]any) {
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return
	}
	br := map[string]any{
		"description": "Bad Request",
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"},
				"example": map[string]any{
					"status_code": 400,
					"status":      "Bad Request",
					"code":        8,
					"error":       "field must be one of [foo bar baz]",
					"request_id":  "579f33bf50b1/abc-000001",
				},
			},
		},
	}
	for _, p := range paths {
		node, ok := p.(map[string]any)
		if !ok {
			continue
		}
		for _, opAny := range node {
			op, ok := opAny.(map[string]any)
			if !ok {
				continue
			}
			resps, ok := op["responses"].(map[string]any)
			if !ok {
				resps = map[string]any{}
				op["responses"] = resps
			}
			if _, exists := resps["400"]; !exists {
				resps["400"] = br
			}
		}
	}
}
