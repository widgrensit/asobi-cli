-- Arena bot AI
-- Called by asobi_bot via think(bot_id, state) each tick

names = {"Spark", "Blitz", "Volt", "Neon", "Pulse"}

local SHOOT_RANGE = 200
local DODGE_CHANCE = 15

local function find_nearest(bot_id, players)
    local me = players[bot_id]
    if not me then return nil, nil, nil end

    local my_x = me.x or 400
    local my_y = me.y or 300
    local best_x, best_y, best_dist = nil, nil, 99999

    for id, p in pairs(players) do
        if id ~= bot_id and p.hp and p.hp > 0 then
            local dx = (p.x or 0) - my_x
            local dy = (p.y or 0) - my_y
            local dist = math.sqrt(dx * dx + dy * dy)
            if dist < best_dist then
                best_dist = dist
                best_x = p.x
                best_y = p.y
            end
        end
    end

    return best_x, best_y, best_dist
end

function think(bot_id, state)
    local players = state.players or {}
    local me = players[bot_id]
    if not me then return {} end

    local my_x = me.x or 400
    local my_y = me.y or 300

    local tx, ty, dist = find_nearest(bot_id, players)

    -- No target: wander randomly
    if not tx then
        return {
            right = math.random(2) == 1,
            left = math.random(2) == 1,
            down = math.random(2) == 1,
            up = math.random(2) == 1,
            shoot = false
        }
    end

    -- Dodge: move perpendicular instead of toward target
    local dodge = math.random(100) <= DODGE_CHANCE
    local move_right, move_left, move_down, move_up

    if dodge then
        move_right = (ty - my_y) > 0
        move_left = (ty - my_y) < 0
        move_up = (tx - my_x) > 0
        move_down = (tx - my_x) < 0
    else
        move_right = tx > my_x
        move_left = tx < my_x
        move_up = ty > my_y
        move_down = ty < my_y
    end

    local shoot = dist <= SHOOT_RANGE
    local aim_x = tx + (math.random(20) - 10)
    local aim_y = ty + (math.random(20) - 10)

    return {
        right = move_right,
        left = move_left,
        down = move_down,
        up = move_up,
        shoot = shoot,
        aim_x = aim_x,
        aim_y = aim_y
    }
end
