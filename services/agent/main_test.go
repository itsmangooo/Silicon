package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestControlPlaneURLRequiresTLS(t *testing.T) {
	if _, err := validateControlPlaneURL("http://silicon.example", false); err == nil {
		t.Fatal("plain HTTP was accepted")
	}
	if _, err := validateControlPlaneURL("https://silicon.example", false); err != nil {
		t.Fatal(err)
	}
	if _, err := validateControlPlaneURL("http://127.0.0.1:8080", true); err != nil {
		t.Fatal(err)
	}
	if _, err := validateControlPlaneURL("https://user:password@silicon.example", false); err == nil {
		t.Fatal("URL credentials were accepted")
	}
}

func TestConfigurationRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "agent.json")
	want := configuration{ControlPlaneURL: "https://silicon.example", AgentID: "agent", ServerID: "server", OrganizationID: "organization", Credential: "machine-secret"}
	if err := saveConfiguration(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfiguration(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("config=%#v want=%#v", got, want)
	}
	if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatalf("configuration permissions are not private: %v %v", info, err)
	}
}
