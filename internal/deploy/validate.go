package deploy

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/widgrensit/asobi-cli/internal/client"
)

// Validate checks a collected bundle the way the engine will load it, so a
// broken bundle fails at the developer's machine with a clear message instead
// of a silently-dead environment (no config.lua found -> guest_auth off and no
// game modes, with nothing surfaced until you connect a client).
func Validate(scripts []client.Script) error {
	if err := validateEntryPoint(scripts); err != nil {
		return err
	}
	if err := validateSyntax(scripts); err != nil {
		return err
	}
	return validateConfig(scripts)
}

// 1. An entry point (config.lua for multi-mode, or match.lua for single-mode)
// must sit at the bundle root - the engine only looks there. This is the
// "deployed the wrong directory, so config.lua is nested" mistake.
func validateEntryPoint(scripts []client.Script) error {
	for _, s := range scripts {
		if s.Path == "config.lua" || s.Path == "match.lua" {
			return nil
		}
	}
	paths := make([]string, 0, len(scripts))
	for _, s := range scripts {
		paths = append(paths, s.Path)
	}
	sort.Strings(paths)
	return fmt.Errorf(
		"no config.lua or match.lua at the bundle root.\n"+
			"Deploy the directory that CONTAINS your entry point, so config.lua "+
			"(or match.lua) is at the root of the bundle.\nFound: %s",
		strings.Join(paths, ", "),
	)
}

// 2. Every .lua file must compile - catches syntax errors (e.g. a missing
// comma in a table) before they silently break the deploy.
func validateSyntax(scripts []client.Script) error {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	for _, s := range scripts {
		if _, err := L.Load(strings.NewReader(s.Content), s.Path); err != nil {
			return fmt.Errorf("%s: Lua syntax error: %v", s.Path, err)
		}
	}
	return nil
}

// 3. Load config.lua like the engine does and check the manifest contract:
// it must return a table of mode_name = "script.lua", each script must exist in
// the bundle, and guest_auth must be a top-level global (not a table field -
// the engine reads it as a global, so a field is silently ignored).
func validateConfig(scripts []client.Script) error {
	var config *client.Script
	present := make(map[string]bool, len(scripts))
	for i := range scripts {
		present[scripts[i].Path] = true
		if scripts[i].Path == "config.lua" {
			config = &scripts[i]
		}
	}
	if config == nil {
		return nil // single-mode (match.lua); no manifest to validate
	}

	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	// Only safe libs - a manifest needs none of os/io/package.
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(lib.fn))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	L.SetContext(ctx)

	fn, err := L.Load(strings.NewReader(config.Content), "config.lua")
	if err != nil {
		return fmt.Errorf("config.lua: %v", err)
	}
	L.Push(fn)
	if err := L.PCall(0, 1, nil); err != nil {
		return fmt.Errorf("config.lua failed to load: %v", err)
	}
	tbl, ok := L.Get(-1).(*lua.LTable)
	if !ok {
		return fmt.Errorf(
			"config.lua must `return` a table of mode_name = \"script.lua\"; got %s",
			L.Get(-1).Type(),
		)
	}

	if tbl.RawGetString("guest_auth") != lua.LNil {
		return fmt.Errorf(
			"config.lua: `guest_auth` must be a top-level global (guest_auth = true), " +
				"not a field inside the returned table - the engine reads it as a global, " +
				"so a table field is silently ignored",
		)
	}

	var errs []string
	tbl.ForEach(func(k, v lua.LValue) {
		name, okName := k.(lua.LString)
		path, okPath := v.(lua.LString)
		if !okName || !okPath {
			errs = append(errs, fmt.Sprintf(
				"entry %v = %v: expected mode_name = \"script.lua\"", k, v))
			return
		}
		if !present[string(path)] {
			errs = append(errs, fmt.Sprintf(
				"mode %q points to %q, which is not in the bundle", string(name), string(path)))
		}
	})
	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("config.lua:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
