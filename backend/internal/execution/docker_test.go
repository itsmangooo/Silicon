package execution

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractGitHubArchiveStripsRootAndRejectsTraversal(t *testing.T) {
	valid := archiveFixture(t, map[string]string{"owner-repo-sha/Dockerfile": "FROM scratch\n", "owner-repo-sha/bin/start": "#!/bin/sh\n"})
	destination := t.TempDir()
	if err := extractGitHubArchive(bytes.NewReader(valid), destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "Dockerfile"))
	if err != nil || string(data) != "FROM scratch\n" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	unsafe := archiveFixture(t, map[string]string{"owner-repo-sha/../../escape": "bad"})
	if err := extractGitHubArchive(bytes.NewReader(unsafe), t.TempDir()); err == nil {
		t.Fatal("unsafe archive path accepted")
	}
}

func archiveFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gz)
	for name, content := range files {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
