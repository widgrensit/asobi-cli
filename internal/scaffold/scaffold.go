package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

const matchLua = `-- match.lua - a minimal server-authoritative Asobi game mode.
-- The engine loads this file as the "default" game mode and drives the
-- callbacks below. See https://github.com/widgrensit/asobi_lua for the
-- full game.* API (economy, leaderboards, storage, spatial, ...).

-- Players needed before a match starts. Required.
match_size = 2

-- init builds the initial match state.
function init(config)
    return { players = {}, tick_count = 0 }
end

-- join runs when a player enters the match.
function join(player_id, state)
    state.players[player_id] = { x = 0, y = 0 }
    return state
end

-- leave runs when a player disconnects.
function leave(player_id, state)
    state.players[player_id] = nil
    return state
end

-- handle_input applies a client message to the state.
function handle_input(player_id, input, state)
    local player = state.players[player_id]
    if player and input then
        player.x = (player.x or 0) + (input.dx or 0)
        player.y = (player.y or 0) + (input.dy or 0)
    end
    return state
end

-- tick advances the simulation (~10 times per second).
function tick(state)
    state.tick_count = (state.tick_count or 0) + 1
    return state
end

-- get_state returns the view sent to a given player.
function get_state(player_id, state)
    return state
end
`

const readmeMd = `# My Asobi game

A minimal server-authoritative multiplayer game running on
[Asobi](https://github.com/widgrensit/asobi).

The game logic lives in ` + "`lua/match.lua`" + `. The engine loads it as the
` + "`default`" + ` game mode and drives the callbacks ` + "`init`, `join`, `leave`," + `
` + "`handle_input`, `tick` and `get_state`. `match_size`" + ` sets how many players
are needed to start a match.

## Run it locally

No account, no keys - runs the asobi_lua image + Postgres in Docker:

` + "```bash" + `
asobi dev
` + "```" + `

Edit ` + "`lua/`" + ` and it hot-reloads. API + WebSocket on http://localhost:8084.

## Deploy to Asobi Cloud

Managed hosting - this needs a login:

` + "```bash" + `
asobi login
asobi use <game>       # list yours: asobi games
asobi create <env>     # e.g. asobi create prod
asobi deploy <env> lua
` + "```" + `

## Multiple game modes

To run more than the ` + "`default`" + ` mode, add a ` + "`lua/config.lua`" + ` manifest
mapping mode names to files:

` + "```lua" + `
return {
    default = "match.lua",
    ranked  = "ranked.lua"
}
` + "```" + `

Clients pick a mode with ` + "`matchmaker.add {mode = \"ranked\"}`" + `.

## A full backend example

Want the whole picture - Docker Compose, the mode manifest, all the
` + "`ASOBI_*`" + ` env vars in one runnable repo?

` + "```bash" + `
asobi init mybackend --template backend
` + "```" + `

## Connect a client

Defold quickstart: https://github.com/widgrensit/asobi-defold
`

type file struct {
	path    string
	content string
}

// Init scaffolds a minimal working server-Lua game into dir. It returns the
// list of created file paths (relative to dir). It refuses to overwrite any
// existing target file, in particular an existing lua/match.lua.
func Init(dir string) ([]string, error) {
	files := []file{
		{filepath.Join("lua", "match.lua"), matchLua},
		{"README.md", readmeMd},
	}

	for _, f := range files {
		full := filepath.Join(dir, f.path)
		if _, err := os.Stat(full); err == nil {
			return nil, fmt.Errorf("%s already exists; refusing to overwrite", full)
		}
	}

	var created []string
	for _, f := range files {
		full := filepath.Join(dir, f.path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return created, fmt.Errorf("create dir for %s: %w", f.path, err)
		}
		if err := os.WriteFile(full, []byte(f.content), 0o644); err != nil {
			return created, fmt.Errorf("write %s: %w", f.path, err)
		}
		created = append(created, f.path)
	}
	return created, nil
}
