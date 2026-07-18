# My Asobi chat room

A real-time chat room on [Asobi](https://github.com/widgrensit/asobi): everyone
who joins shares one room, and messages are relayed by the server so the author
can't be forged.

The logic lives in `lua/match.lua`:

- `handle_input` takes `{say = "hello"}` and broadcasts it as a `chat` event,
  stamping the author from `player_id`.
- `join` / `leave` announce arrivals and departures.
- `match_size` opens the room at 2 players; `max_players` lets it fill to 50.

Clients listen for the `chat` event and send `{say = "..."}` inputs.

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

- Persist history with `game.chat.send`: https://hexdocs.pm/asobi/lua-scripting.html
- Connect a client - Defold quickstart: https://github.com/widgrensit/asobi-defold
