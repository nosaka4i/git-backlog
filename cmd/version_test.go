package cmd

import (
	"strings"
	"testing"
)

func TestVersionPrintsResolveVersion(t *testing.T) {
	out, err := runCmd(t, newVersionCmd())
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != resolveVersion() {
		t.Fatalf("output = %q, want %q", out, resolveVersion())
	}
}

func TestResolveVersionPrefersExplicitOverride(t *testing.T) {
	orig := version
	defer func() { version = orig }()

	version = "v1.2.3"
	if got := resolveVersion(); got != "v1.2.3" {
		t.Fatalf("resolveVersion() = %q, want v1.2.3", got)
	}
}

func TestResolveVersionFallsBackToDev(t *testing.T) {
	orig := version
	defer func() { version = orig }()

	version = "dev"
	// go test builds don't carry a meaningful module version, so this
	// should fall back to "dev" rather than panic or return "".
	if got := resolveVersion(); got == "" {
		t.Fatal("resolveVersion() returned empty string")
	}
}
