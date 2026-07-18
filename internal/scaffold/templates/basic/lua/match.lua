-- match.lua - a minimal server-authoritative Asobi game mode.
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
