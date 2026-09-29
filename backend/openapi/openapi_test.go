package openapi_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDocumentParsesAndInternalReferencesResolve(t *testing.T) {
	raw, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse OpenAPI YAML: %v", err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("openapi version=%v want 3.1.0", document["openapi"])
	}
	paths, ok := document["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		t.Fatal("OpenAPI document has no paths")
	}
	for path := range paths {
		if !strings.HasPrefix(path, "/") {
			t.Fatalf("invalid OpenAPI path %q", path)
		}
	}
	walkReferences(t, document, document)
}

func walkReferences(t *testing.T, root map[string]any, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "$ref" {
				reference, ok := child.(string)
				if !ok || !strings.HasPrefix(reference, "#/") {
					t.Fatalf("unsupported OpenAPI reference %v", child)
				}
				if !resolves(root, reference) {
					t.Fatalf("unresolved OpenAPI reference %s", reference)
				}
				continue
			}
			walkReferences(t, root, child)
		}
	case []any:
		for _, child := range typed {
			walkReferences(t, root, child)
		}
	}
}

func resolves(root map[string]any, reference string) bool {
	var current any = root
	for _, segment := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		segment = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
		current, ok = object[segment]
		if !ok {
			return false
		}
	}
	return true
}
