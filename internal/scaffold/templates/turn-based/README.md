# My Asobi turn-based game

A turn-based game on [Asobi](https://github.com/widgrensit/asobi): two players
alternate claiming cells on a 3x3 board. The server enforces turn order, so an
out-of-turn or duplicate move is rejected rather than trusted from the client.

The logic lives in `lua/match.lua`:

- `handle_input` claims a cell only if it is the sender's turn and the cell is
  free, then passes the turn and broadcasts a `move` event.
- `state.turn` and `state.board` are server-owned; clients send `{cell = 1..9}`.
- `match_size` starts the game at 2 players.

There is no `tick` simulation here - the state only changes on a valid move.

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

- Detect a win and end the match by returning a `finished` flag from a callback.
- Connect a client - Defold quickstart: https://github.com/widgrensit/asobi-defold
