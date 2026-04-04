# asobi-cli

Command-line tool for building multiplayer games on [Asobi](https://github.com/widgrensit/asobi). Scaffold projects, run tests, manage your game server.

## Install

```bash
git clone https://github.com/widgrensit/asobi-cli
ln -s $(pwd)/asobi-cli/bin/asobi ~/bin/asobi
```

Requires [Lua 5.3+](https://www.lua.org/) and [Docker](https://www.docker.com/).

## Usage

### Create a new game

```bash
asobi new minimal my-game     # bare-bones starting point
asobi new arena my-shooter    # full arena shooter with boons, modifiers, bot AI
```

### Develop

```bash
cd my-game
asobi dev       # start the game server (docker compose up)
asobi test      # run Lua tests locally
asobi state     # show active matches
asobi logs      # tail server logs
asobi stop      # stop the server
```

### Write tests

Create `*_test.lua` files in `test/`. Any function starting with `test_` is a test case. Use Lua's built-in `assert()` for assertions.

```lua
-- test/match_test.lua
local boons = require("boons")

function test_hp_boost()
    local stats = boons.default_stats()
    stats = boons.apply("hp_boost", stats)
    assert(stats.max_hp == 115, "hp_boost should add 15")
end
```

```bash
asobi test                # run all tests
asobi test test/my_test.lua  # run specific file
```

## Templates

| Template | Description |
|----------|-------------|
| `minimal` | Empty game with movement. Start here to build from scratch. |
| `arena` | Top-down arena shooter with boons, modifiers, voting, and bot AI. |

## Commands

| Command | Description |
|---------|-------------|
| `asobi new <template> <name>` | Scaffold a new game project |
| `asobi dev` | Start the game server |
| `asobi stop` | Stop the game server |
| `asobi test [file]` | Run Lua tests locally |
| `asobi state` | Show active matches and player counts |
| `asobi logs` | Tail game server logs |
| `asobi help` | Show help |
