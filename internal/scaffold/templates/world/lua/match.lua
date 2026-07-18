-- match.lua - a persistent shared world on Asobi.
-- game_type = "world" routes this script through the world bridge: players share
-- zones of a larger map, the server ticks each zone, and every position is
-- server-owned. World mode has a different callback shape than a match: it uses
-- spawn_position, zone_tick and post_tick, and handle_input returns the zone's
-- entities table (not the whole state). See
-- https://github.com/widgrensit/asobi_lua for the full world.* / spatial.* API.

game_type = "world"
match_size = 1
max_players = 100
tick_rate = 50

function init(config)
    return { tick_count = 0 }
end

function join(player_id, state)
    return state
end

function leave(player_id, state)
    return state
end

-- spawn_position places a joining player somewhere in the world.
function spawn_position(player_id, state)
    return { x = 0, y = 0 }
end

-- zone_tick advances one zone each tick. entities is everything currently in the
-- zone; return it (optionally mutated) plus the zone state.
function zone_tick(entities, zone_state)
    return entities, zone_state
end

-- handle_input moves a player's entity. In world mode it returns the entities
-- table, not the whole match state.
function handle_input(player_id, input, entities)
    local me = entities[player_id]
    if me and input then
        me.x = (me.x or 0) + (input.dx or 0)
        me.y = (me.y or 0) + (input.dy or 0)
    end
    return entities
end

-- post_tick runs once per world tick, after every zone has ticked.
function post_tick(tick, state)
    state.tick_count = (state.tick_count or 0) + 1
    return state
end

function get_state(player_id, state)
    return state
end
