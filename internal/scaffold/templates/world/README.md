# My Asobi world

A persistent shared world on [Asobi](https://github.com/widgrensit/asobi):
players share zones of a larger map instead of a fixed-size match. Setting
`game_type = "world"` routes the script through the world bridge.

World mode has a different callback shape than a match:

- `spawn_position` places a joining player in the world.
- `zone_tick` advances one zone's entities each tick.
- `handle_input` returns the zone's `entities` table, not the whole state.
- `post_tick` runs once per world tick after every zone has ticked.
- `tick_rate` is the world tick interval in ms (50 = 20 Hz).

The logic lives in `lua/match.lua`. Zone spawning and terrain are available via
`game.zone.*` and `game.terrain.*` once you build the world out.

## Run it locally

No account, no keys - runs the asobi_lua image + Postgres in Docker:

```bash
asobi dev
```

Edit `lua/` and it hot-reloads. API + WebSocket on http://localhost:8084.

## Deploy to Asobi Cloud

```bash
asobi login
asobi use <game>       # list yours: asobi games
asobi create <env>     # e.g. asobi create prod
asobi deploy <env> lua
```

## Next

- World server guide: https://hexdocs.pm/asobi/world-server.html
- Connect a client - Defold quickstart: https://github.com/widgrensit/asobi-defold
