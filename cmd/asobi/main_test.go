package main

import "testing"

func TestIsHealthy(t *testing.T) {
	for _, s := range []string{"ok", "healthy"} {
		if !isHealthy(s) {
			t.Errorf("isHealthy(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "degraded", "unhealthy", "OK"} {
		if isHealthy(s) {
			t.Errorf("isHealthy(%q) = true, want false", s)
		}
	}
}

func TestFirstArg(t *testing.T) {
	// firstArg runs on args after extractFlag has stripped known flags and
	// their values, so it only skips bare "--" tokens and returns the first
	// positional.
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"--verbose"}, ""},
		{[]string{"prod"}, "prod"},
		{[]string{"--verbose", "prod"}, "prod"},
		{[]string{"prod", "--verbose"}, "prod"},
	}
	for _, c := range cases {
		if got := firstArg(c.args); got != c.want {
			t.Errorf("firstArg(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestSelectEnv(t *testing.T) {
	prod := map[string]interface{}{"name": "prod"}
	staging := map[string]interface{}{"name": "staging"}

	if _, err := selectEnv(nil, ""); err == nil {
		t.Error("selectEnv(empty) should error")
	}

	got, err := selectEnv([]map[string]interface{}{prod}, "")
	if err != nil || got["name"] != "prod" {
		t.Errorf("single env: got %v, err %v", got, err)
	}

	got, err = selectEnv([]map[string]interface{}{prod, staging}, "staging")
	if err != nil || got["name"] != "staging" {
		t.Errorf("by name: got %v, err %v", got, err)
	}

	if _, err := selectEnv([]map[string]interface{}{prod, staging}, "nope"); err == nil {
		t.Error("unknown name should error")
	}

	if _, err := selectEnv([]map[string]interface{}{prod, staging}, ""); err == nil {
		t.Error("multiple envs without a name should be ambiguous")
	}
}
