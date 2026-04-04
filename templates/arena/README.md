# {{PROJECT_NAME}}

A top-down multiplayer arena shooter built on [Asobi](https://github.com/widgrensit/asobi).

## Quick Start

```bash
asobi dev       # start the game server
asobi test      # run tests
asobi stop      # stop the server
```

## Game Rules

- **Arena**: 800x600, up to 10 players
- **Rounds**: 90 seconds, ends early if one player remains
- **Combat**: Projectile-based with collision detection
- **Boons**: Top 3 players pick upgrades between rounds
- **Modifiers**: All players vote on next round's modifier
- **Bots**: Fill matches automatically with Lua AI

## Project Structure

```
lua/
  match.lua          # Game logic (callbacks)
  boons.lua          # 7 power-ups
  modifiers.lua      # 6 round modifiers
  config.lua         # Game mode manifest
  bots/
    arena_bot.lua    # Bot AI (chase, dodge, shoot)
test/
  match_test.lua     # Tests
```

## Customization

- Edit `lua/boons.lua` to add/modify power-ups
- Edit `lua/modifiers.lua` to add round modifiers
- Edit `lua/bots/arena_bot.lua` to change bot behavior
- Edit `lua/match.lua` to change game rules
