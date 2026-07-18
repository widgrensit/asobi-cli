# My Asobi game

A minimal server-authoritative multiplayer game running on
[Asobi](https://github.com/widgrensit/asobi).

The game logic lives in `lua/match.lua`. The engine loads it as the
`default` game mode and drives the callbacks `init`, `join`, `leave`,
`handle_input`, `tick` and `get_state`. `match_size` sets how many players
are needed to start a match.

## Run it locally

No account, no keys - runs the asobi_lua image + Postgres in Docker:

```bash
asobi dev
```

Edit `lua/` and it hot-reloads. API + WebSocket on http://localhost:8084.

## Deploy to Asobi Cloud

Managed hosting - this needs a login:

```bash
asobi login
asobi use <game>       # list yours: asobi games
asobi create <env>     # e.g. asobi create prod
asobi deploy <env> lua
```

## Multiple game modes

To run more than the `default` mode, add a `lua/config.lua` manifest
mapping mode names to files:

```lua
return {
    default = "match.lua",
    ranked  = "ranked.lua"
}
```

Clients pick a mode with `matchmaker.add {mode = "ranked"}`.

## A full backend example

Want the whole picture - Docker Compose, the mode manifest, all the
`ASOBI_*` env vars in one runnable repo?

```bash
asobi init mybackend --template backend
```

## Connect a client

Defold quickstart: https://github.com/widgrensit/asobi-defold
