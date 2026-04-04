-- Arena shooter match logic
-- Implements all asobi_match callbacks in Lua

-- Game mode config (read by asobi at startup)
match_size = 4
max_players = 10
strategy = "fill"
bots = { script = "bots/arena_bot.lua" }

local boons = require("boons")
local modifiers = require("modifiers")

-- Constants
local ARENA_W = 800
local ARENA_H = 600
local GAME_DURATION = 90000
local TICK_MS = 100
local PLAYER_RADIUS = 16
local BOON_PICK_TIMEOUT = 15000
local TOP_N = 3

-- Helpers

local function clamp(v, min, max)
    if v < min then return min end
    if v > max then return max end
    return v
end

local function distance(x1, y1, x2, y2)
    local dx = x2 - x1
    local dy = y2 - y1
    return math.sqrt(dx * dx + dy * dy)
end

local function now_ms()
    -- Luerl doesn't expose os.clock in ms, so we track time via tick counting
    -- Each tick is 100ms; we'll use a counter-based approach
    return nil
end

local function count_keys(t)
    local n = 0
    for _ in pairs(t) do n = n + 1 end
    return n
end

local function is_bot(id)
    return type(id) == "string" and string.sub(id, 1, 4) == "bot_"
end

local function shallow_copy(t)
    local out = {}
    for k, v in pairs(t) do out[k] = v end
    return out
end

local function copy_list(t)
    local out = {}
    for i, v in ipairs(t) do out[i] = v end
    return out
end

-- Effective stat helpers (apply modifier config)

local function effective_hp(stats, mod_config)
    local base = stats.max_hp
    if mod_config.hp_override then
        return math.min(mod_config.hp_override, base)
    end
    return base
end

local function effective_damage(stats, mod_config)
    return stats.damage * (mod_config.damage_mult or 1)
end

local function effective_proj_speed(stats, mod_config)
    return stats.projectile_speed * (mod_config.projectile_speed_mult or 1)
end

local function effective_cooldown(stats, mod_config)
    local base = stats.shoot_cooldown
    local mult = mod_config.cooldown_mult or 1
    return math.floor(base * mult + 0.5)
end

-- Build standings from players (sorted by kills descending)

local function build_standings(players)
    local list = {}
    for id, p in pairs(players) do
        table.insert(list, { player_id = id, kills = p.kills or 0, deaths = p.deaths or 0 })
    end
    table.sort(list, function(a, b) return a.kills > b.kills end)
    for i, s in ipairs(list) do
        s.rank = i
    end
    return list
end

local function top_player_ids(standings, n)
    local ids = {}
    for i = 1, math.min(n, #standings) do
        table.insert(ids, standings[i].player_id)
    end
    return ids
end

-- Generate boon offers for top players

local function generate_boon_offers(top_ids, session_stats)
    local offers = {}
    for _, pid in ipairs(top_ids) do
        local stats = session_stats[pid] or boons.default_stats()
        local already = stats.boons or {}
        local choices = boons.random_choices(3, already)
        if #choices > 0 then
            offers[pid] = choices
        end
    end
    return offers
end

-- Build result for match finish

local function build_result(state)
    local standings = build_standings(state.players)
    local winner = nil
    if #standings > 0 then
        winner = standings[1].player_id
    end
    return {
        status = "completed",
        standings = standings,
        round = state.round,
        session_stats = state.session_stats,
        winner = winner
    }
end

-- Auto-pick boons for players who haven't picked

local function auto_pick_remaining(state)
    for pid, offer_list in pairs(state.boon_offers) do
        if not state.boon_picks[pid] and #offer_list > 0 then
            local pick_id = offer_list[1].id
            local stats = state.session_stats[pid] or boons.default_stats()
            state.session_stats[pid] = boons.apply(pick_id, stats)
            state.boon_picks[pid] = pick_id
        end
    end
    return state
end

-- Callbacks

function init(config)
    config = config or {}
    local modifier = config.modifier
    local mod_config = modifiers.apply(modifier)
    return {
        players = {},
        projectiles = {},
        next_proj_id = 1,
        tick_count = 0,
        phase = "playing",
        modifier = modifier,
        mod_config = mod_config,
        session_stats = config.session_stats or {},
        round = config.round or 1,
        boon_offers = {},
        boon_picks = {},
        boon_pick_deadline = 0,
        standings = {},
        arena_w = mod_config.arena_w or ARENA_W,
        arena_h = mod_config.arena_h or ARENA_H
    }
end

function join(player_id, state)
    local stats = state.session_stats[player_id] or boons.default_stats()
    local mod_config = state.mod_config or {}
    local max_hp = effective_hp(stats, mod_config)
    local w = state.arena_w
    local h = state.arena_h

    state.players[player_id] = {
        x = math.random(w - 100) + 50,
        y = math.random(h - 100) + 50,
        hp = max_hp,
        max_hp = max_hp,
        kills = 0,
        deaths = 0,
        shoot_cd = 0,
        speed = stats.speed,
        damage = effective_damage(stats, mod_config),
        projectile_speed = effective_proj_speed(stats, mod_config),
        projectile_radius = stats.projectile_radius,
        shoot_cooldown = effective_cooldown(stats, mod_config),
        lifesteal = stats.lifesteal or 0,
        lives = mod_config.lives or 0,
        boons = copy_list(stats.boons or {})
    }

    return state
end

function leave(player_id, state)
    state.players[player_id] = nil
    return state
end

function handle_input(player_id, input, state)
    -- Boon pick
    if input.type == "boon_pick" and input.boon_id then
        if state.phase ~= "boon_pick" then return state end
        local offers = state.boon_offers[player_id]
        if not offers then return state end
        if state.boon_picks[player_id] then return state end

        -- Validate boon is in offers
        local valid = false
        for _, boon in ipairs(offers) do
            if boon.id == input.boon_id then
                valid = true
                break
            end
        end
        if not valid then return state end

        local stats = state.session_stats[player_id] or boons.default_stats()
        state.session_stats[player_id] = boons.apply(input.boon_id, stats)
        state.boon_picks[player_id] = input.boon_id
        return state
    end

    -- Playing phase only
    if state.phase ~= "playing" then return state end
    local p = state.players[player_id]
    if not p or p.hp <= 0 then return state end

    -- Movement
    local dx = 0
    local dy = 0
    if input.right then dx = dx + p.speed end
    if input.left then dx = dx - p.speed end
    if input.up then dy = dy + p.speed end
    if input.down then dy = dy - p.speed end

    p.x = clamp(p.x + dx, PLAYER_RADIUS, state.arena_w - PLAYER_RADIUS)
    p.y = clamp(p.y + dy, PLAYER_RADIUS, state.arena_h - PLAYER_RADIUS)

    -- Push apart from other players
    local min_dist = PLAYER_RADIUS * 2
    for other_id, other in pairs(state.players) do
        if other_id ~= player_id and other.hp > 0 then
            local sep_dx = p.x - other.x
            local sep_dy = p.y - other.y
            local dist = math.sqrt(sep_dx * sep_dx + sep_dy * sep_dy)
            if dist < min_dist and dist > 0 then
                local overlap = min_dist - dist
                local nx = sep_dx / dist
                local ny = sep_dy / dist
                p.x = clamp(p.x + nx * overlap, PLAYER_RADIUS, state.arena_w - PLAYER_RADIUS)
                p.y = clamp(p.y + ny * overlap, PLAYER_RADIUS, state.arena_h - PLAYER_RADIUS)
            end
        end
    end

    -- Shooting
    if input.shoot and input.aim_x and input.aim_y and p.shoot_cd <= 0 then
        local aim_dx = input.aim_x - p.x
        local aim_dy = input.aim_y - p.y
        local len = math.sqrt(aim_dx * aim_dx + aim_dy * aim_dy)
        if len > 0 then
            local proj = {
                id = state.next_proj_id,
                x = p.x,
                y = p.y,
                vx = (aim_dx / len) * p.projectile_speed,
                vy = (aim_dy / len) * p.projectile_speed,
                radius = p.projectile_radius,
                owner = player_id
            }
            table.insert(state.projectiles, proj)
            state.next_proj_id = state.next_proj_id + 1
            p.shoot_cd = p.shoot_cooldown
        end
    end

    state.players[player_id] = p
    return state
end

function tick(state)
    state.tick_count = state.tick_count + 1

    -- Boon pick phase: check deadline
    if state.phase == "boon_pick" then
        local ticks_since_deadline = state.tick_count - state.boon_pick_deadline
        if ticks_since_deadline >= 0 then
            state = auto_pick_remaining(state)
            state._finished = true
            state._result = build_result(state)
            return state
        end

        -- Check if all human players have picked (or only bots remain)
        local all_done = true
        local has_human_pick = false
        for pid, _ in pairs(state.boon_offers) do
            if state.boon_picks[pid] then
                if not is_bot(pid) then has_human_pick = true end
            elseif not is_bot(pid) then
                all_done = false
            end
        end

        if all_done and (has_human_pick or count_keys(state.boon_picks) > 0) then
            state = auto_pick_remaining(state)
            state._finished = true
            state._result = build_result(state)
            return state
        end

        return state
    end

    -- Playing phase

    -- Move projectiles
    local new_projs = {}
    for _, proj in ipairs(state.projectiles) do
        proj.x = proj.x + proj.vx
        proj.y = proj.y + proj.vy
        table.insert(new_projs, proj)
    end

    -- Collision detection
    local surviving_projs = {}
    for _, proj in ipairs(new_projs) do
        local hit = false
        local hit_id = nil

        for pid, p in pairs(state.players) do
            if pid ~= proj.owner and p.hp > 0 then
                local hit_radius = PLAYER_RADIUS + proj.radius
                local dist = distance(proj.x, proj.y, p.x, p.y)
                if dist <= hit_radius then
                    -- Get owner's damage
                    local owner = state.players[proj.owner]
                    local dmg = owner and owner.damage or 25
                    p.hp = math.max(0, p.hp - dmg)
                    if p.hp == 0 then
                        p.deaths = p.deaths + 1
                    end
                    state.players[pid] = p
                    hit = true
                    hit_id = pid
                    break
                end
            end
        end

        if hit then
            -- Credit kill
            local owner = state.players[proj.owner]
            if owner and owner.hp > 0 then
                local target = state.players[hit_id]
                if target and target.hp == 0 then
                    owner.kills = owner.kills + 1
                end
                -- Lifesteal
                if owner.lifesteal > 0 then
                    local heal = math.floor(owner.damage * owner.lifesteal / 100 + 0.5)
                    owner.hp = math.min(owner.max_hp, owner.hp + heal)
                end
                state.players[proj.owner] = owner
            end
        else
            -- Keep projectile if in bounds
            if proj.x >= 0 and proj.x <= state.arena_w and proj.y >= 0 and proj.y <= state.arena_h then
                table.insert(surviving_projs, proj)
            end
        end
    end
    state.projectiles = surviving_projs

    -- Tick cooldowns
    for pid, p in pairs(state.players) do
        if p.shoot_cd > 0 then
            p.shoot_cd = p.shoot_cd - 1
        end
    end

    -- Respawn
    for pid, p in pairs(state.players) do
        if p.hp == 0 and p.lives > 0 then
            p.hp = p.max_hp
            p.lives = p.lives - 1
            p.x = math.random(state.arena_w - 100) + 50
            p.y = math.random(state.arena_h - 100) + 50
            p.shoot_cd = 30
        end
    end

    -- Check finish condition
    local elapsed = state.tick_count * TICK_MS
    local num_players = count_keys(state.players)
    local num_alive = 0
    for _, p in pairs(state.players) do
        if p.hp > 0 or p.lives > 0 then
            num_alive = num_alive + 1
        end
    end

    local finished = false
    if elapsed >= GAME_DURATION then
        finished = true
    elseif num_players >= 2 and num_alive <= 1 then
        finished = true
    end

    if finished then
        local standings = build_standings(state.players)
        state.standings = standings
        local top_ids = top_player_ids(standings, TOP_N)
        local offers = generate_boon_offers(top_ids, state.session_stats)

        if count_keys(offers) == 0 then
            state.phase = "vote_pending"
        else
            state.phase = "boon_pick"
            state.boon_offers = offers
            state.boon_picks = {}
            state.boon_pick_deadline = state.tick_count + (BOON_PICK_TIMEOUT / TICK_MS)
        end
    end

    return state
end

function get_state(player_id, state)
    if state.phase == "boon_pick" then
        local my_offers = state.boon_offers[player_id] or {}
        local offer_list = {}
        for _, b in ipairs(my_offers) do
            table.insert(offer_list, { id = b.id, name = b.name, description = b.description })
        end
        local picks_done = {}
        for pid, _ in pairs(state.boon_picks) do
            table.insert(picks_done, pid)
        end
        local remaining = (state.boon_pick_deadline - state.tick_count) * TICK_MS
        return {
            phase = "boon_pick",
            standings = state.standings,
            boon_offers = offer_list,
            picks_done = picks_done,
            time_remaining = math.max(0, remaining)
        }
    end

    if state.phase == "voting" then
        return {
            phase = "voting",
            standings = state.standings,
            current_modifier = state.modifier
        }
    end

    -- Playing phase
    local player_states = {}
    for pid, p in pairs(state.players) do
        player_states[pid] = {
            x = p.x,
            y = p.y,
            hp = p.hp,
            max_hp = p.max_hp,
            kills = p.kills,
            deaths = p.deaths,
            boons = p.boons
        }
    end

    local proj_states = {}
    for _, proj in ipairs(state.projectiles) do
        table.insert(proj_states, {
            id = proj.id,
            x = proj.x,
            y = proj.y,
            owner = proj.owner
        })
    end

    local elapsed = state.tick_count * TICK_MS
    local my_boons = {}
    local me = state.players[player_id]
    if me then
        my_boons = me.boons or {}
    end

    return {
        phase = "playing",
        players = player_states,
        projectiles = proj_states,
        time_remaining = math.max(0, GAME_DURATION - elapsed),
        arena_w = state.arena_w,
        arena_h = state.arena_h,
        modifier = state.modifier,
        round = state.round,
        my_boons = my_boons
    }
end

function vote_requested(state)
    if state.phase == "vote_pending" then
        return {
            template = "arena_modifier",
            options = modifiers.vote_options(),
            method = "plurality",
            window_ms = 15000,
            visibility = "live"
        }
    end
    return nil
end

function vote_resolved(template, result, state)
    if template == "arena_modifier" then
        -- Ensure all players have session stats
        for pid, _ in pairs(state.players) do
            if not state.session_stats[pid] then
                state.session_stats[pid] = boons.default_stats()
            end
        end
        state.phase = "finished"
        state.round = state.round + 1
        state.next_modifier = result.winner

        state._finished = true
        state._result = build_result(state)
    end
    return state
end
