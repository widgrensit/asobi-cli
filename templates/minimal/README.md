# {{PROJECT_NAME}}

A multiplayer game built on [Asobi](https://github.com/widgrensit/asobi).

## Quick Start

```bash
asobi dev       # start the game server
asobi test      # run tests
asobi stop      # stop the server
```

## Project Structure

```
lua/
  match.lua       # Game logic (all callbacks)
  bots/bot.lua    # Bot AI
test/
  match_test.lua  # Tests
```

## Game Logic

Edit `lua/match.lua` to build your game. Implement the callbacks:

- `init(config)` - create initial game state
- `join(player_id, state)` - handle player joining
- `leave(player_id, state)` - handle player leaving
- `handle_input(player_id, input, state)` - process player input
- `tick(state)` - game loop (10 times/sec)
- `get_state(player_id, state)` - what each player sees

See the [Lua scripting guide](https://github.com/widgrensit/asobi/blob/main/guides/lua-scripting.md) for details.
