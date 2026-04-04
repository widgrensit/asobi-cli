-- Minimal Asobi game server
-- Edit this file to build your game logic

-- Game mode config (read by asobi at startup)
match_size = 2
max_players = 8
strategy = "fill"
bots = { script = "bots/bot.lua" }

-- Called once when a match is created
function init(config)
    return {
        players = {},
        tick_count = 0
    }
end

-- Called when a player joins the match
function join(player_id, state)
    state.players[player_id] = {
        x = math.random(800),
        y = math.random(600),
        hp = 100,
        score = 0
    }
    return state
end

-- Called when a player leaves
function leave(player_id, state)
    state.players[player_id] = nil
    return state
end

-- Called when a player sends input via WebSocket
function handle_input(player_id, input, state)
    local p = state.players[player_id]
    if not p then return state end

    local speed = 4
    if input.right then p.x = p.x + speed end
    if input.left then p.x = p.x - speed end
    if input.up then p.y = p.y + speed end
    if input.down then p.y = p.y - speed end

    state.players[player_id] = p
    return state
end

-- Called every tick (10 times per second)
-- Set _finished = true and _result = {...} to end the match
function tick(state)
    state.tick_count = state.tick_count + 1

    -- End match after 90 seconds (900 ticks)
    if state.tick_count >= 900 then
        state._finished = true
        state._result = {
            status = "completed",
            tick_count = state.tick_count
        }
    end

    return state
end

-- Called every tick for each player
-- Return only what this player should see
function get_state(player_id, state)
    return {
        players = state.players,
        tick_count = state.tick_count
    }
end
