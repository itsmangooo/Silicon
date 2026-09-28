package store

import "testing"

func TestEscapeLike(t *testing.T) {
	got := escapeLike(`deploy_100%\latest`)
	want := `deploy\_100\%\\latest`
	if got != want {
		t.Fatalf("escapeLike() = %q, want %q", got, want)
	}
}
