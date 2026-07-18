-- match.lua - a real-time arena game mode on Asobi.
-- Players move every tick and can shoot; the server owns all positions and hp,
-- so no client can cheat its own coordinates or a hit. See
-- https://github.com/widgrensit/asobi_lua for the full game.* API.

match_size = 2
max_players = 8

function init(config)
    return { players = {}, tick_count = 0 }
end

function join(player_id, state)
    state.players[player_id] = { x = 0, y = 0, hp = 100 }
    return state
end

function leave(player_id, state)
    state.players[player_id] = nil
    return state
end

-- handle_input moves the player and, on a shoot input, damages the nearest
-- living enemy in range. Movement and damage are both applied server-side.
function handle_input(player_id, input, state)
    local me = state.players[player_id]
    if not me or me.hp <= 0 or not input then
        return state
    end

    me.x = me.x + (input.dx or 0)
    me.y = me.y + (input.dy or 0)

    if input.shoot then
        local target = nearest_enemy(player_id, me, state.players)
        if target and distance(me, target) < 200 then
            target.hp = math.max(0, target.hp - 25)
        end
    end
    return state
end

function tick(state)
    state.tick_count = (state.tick_count or 0) + 1
    return state
end

function get_state(player_id, state)
    return state
end

function nearest_enemy(id, me, players)
    local best, best_d
    for pid, p in pairs(players) do
        if pid ~= id and p.hp > 0 then
            local d = distance(me, p)
            if not best_d or d < best_d then
                best, best_d = p, d
            end
        end
    end
    return best
end

function distance(a, b)
    local dx, dy = a.x - b.x, a.y - b.y
    return math.sqrt(dx * dx + dy * dy)
end
