package deploy

import (
	"strings"
	"testing"

	"github.com/widgrensit/asobi-cli/internal/client"
)

func mustFail(t *testing.T, scripts []client.Script, wantSubstr string) {
	t.Helper()
	err := Validate(scripts)
	if err == nil {
		t.Fatalf("expected validation error containing %q, got nil", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("error %q does not contain %q", err.Error(), wantSubstr)
	}
}

func TestValidateRejectsNestedEntryPoint(t *testing.T) {
	// The "deployed the wrong directory" mistake: config.lua ends up nested.
	scripts := []client.Script{
		{Path: "grid_hackers/lua/config.lua", Content: "return {}"},
		{Path: "grid_hackers/lua/grid1/match.lua", Content: "return {}"},
	}
	mustFail(t, scripts, "no config.lua or match.lua at the bundle root")
}

func TestValidateRejectsSyntaxError(t *testing.T) {
	// Pen's exact config.lua: guest_auth mislaid AND a missing comma.
	scripts := []client.Script{
		{Path: "config.lua", Content: "return {\n  guest_auth = true\n  grid1 = \"grid1/match.lua\"\n}"},
		{Path: "grid1/match.lua", Content: "return {}"},
	}
	mustFail(t, scripts, "syntax error")
}

func TestValidateRejectsGuestAuthInTable(t *testing.T) {
	// Syntactically valid (comma added) but guest_auth is a table field.
	scripts := []client.Script{
		{Path: "config.lua", Content: "return {\n  guest_auth = true,\n  grid1 = \"grid1/match.lua\"\n}"},
		{Path: "grid1/match.lua", Content: "return {}"},
	}
	mustFail(t, scripts, "guest_auth")
}

func TestValidateRejectsMissingModeScript(t *testing.T) {
	scripts := []client.Script{
		{Path: "config.lua", Content: "return { arena = \"modes/arena.lua\" }"},
	}
	mustFail(t, scripts, "not in the bundle")
}

func TestValidateRejectsNonTableConfig(t *testing.T) {
	scripts := []client.Script{
		{Path: "config.lua", Content: "return 42"},
	}
	mustFail(t, scripts, "must `return` a table")
}

func TestValidateAcceptsValidMultiMode(t *testing.T) {
	scripts := []client.Script{
		{Path: "config.lua", Content: "guest_auth = true\n\nreturn { grid1 = \"grid1/match.lua\" }"},
		{Path: "grid1/match.lua", Content: "return { players = {} }"},
	}
	if err := Validate(scripts); err != nil {
		t.Fatalf("valid multi-mode bundle rejected: %v", err)
	}
}

func TestValidateAcceptsSingleMode(t *testing.T) {
	// No config.lua: match.lua at the root is a valid single-mode bundle.
	scripts := []client.Script{
		{Path: "match.lua", Content: "match_size = 2\nreturn { players = {} }"},
	}
	if err := Validate(scripts); err != nil {
		t.Fatalf("valid single-mode bundle rejected: %v", err)
	}
}
