package configuration

import "testing"

func TestParseDotEnv(t *testing.T) {
	items, err := ParseDotEnv("# ignored\nLOG_LEVEL=info\nexport API_VERSION=\"v1\"\nAPI_TOKEN=not-automatically-secret\nLOG_LEVEL=warn\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Name != "API_TOKEN" || !items[0].SecretSuggested || items[1].Value != "v1" || items[2].Value != "warn" {
		t.Fatalf("unexpected parsed values: %#v", items)
	}
	if _, err = ParseDotEnv("INVALID LINE"); err == nil {
		t.Fatal("invalid .env line was accepted")
	}
}
