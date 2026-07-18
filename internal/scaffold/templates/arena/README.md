# My Asobi arena game

A real-time arena on [Asobi](https://github.com/widgrensit/asobi): players
move each tick and shoot the nearest enemy. The server owns every position and
hit, so clients only send inputs (`dx`, `dy`, `shoot`) and render the state
they get back.

The logic lives in `lua/match.lua`:

- `handle_input` applies movement and a range-checked hit, server-side.
- `tick` runs the simulation loop (~10 Hz).
- `match_size` starts a match at 2 players; `max_players` lets it fill to 8.

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

- Add bots to fill empty slots: https://hexdocs.pm/asobi/lua-bots.html
- Connect a client - Defold quickstart: https://github.com/widgrensit/asobi-defold
