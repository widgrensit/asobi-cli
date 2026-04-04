-- Tests for arena game
-- Run with: asobi test

-- Load game modules
local boons = require("boons")
local modifiers = require("modifiers")

-- Load match in its own environment
local match_env = setmetatable({}, {__index = _G})
local chunk = assert(loadfile("lua/match.lua", "t", match_env))
chunk()

-- Boon tests

function test_default_stats()
    local stats = boons.default_stats()
    assert(stats.max_hp == 100, "default hp should be 100")
    assert(stats.damage == 25, "default damage should be 25")
    assert(stats.speed == 4, "default speed should be 4")
    assert(stats.lifesteal == 0, "default lifesteal should be 0")
end

function test_apply_hp_boost()
    local stats = boons.default_stats()
    stats = boons.apply("hp_boost", stats)
    assert(stats.max_hp == 115, "hp_boost should add 15 to max_hp")
end

function test_apply_cooldown_min()
    local stats = boons.default_stats()
    -- Apply enough cooldown reductions to hit the floor
    stats = boons.apply("short_cooldown", stats) -- 15 - 4 = 11
    stats = boons.apply("short_cooldown", stats) -- 11 - 4 = 7
    stats = boons.apply("short_cooldown", stats) -- 7 - 4 = 5 (clamped)
    assert(stats.shoot_cooldown >= 5, "cooldown should not go below 5")
end

function test_random_choices_excludes_picked()
    local choices = boons.random_choices(7, {"hp_boost", "damage_boost"})
    for _, c in ipairs(choices) do
        assert(c.id ~= "hp_boost", "should exclude hp_boost")
        assert(c.id ~= "damage_boost", "should exclude damage_boost")
    end
    assert(#choices == 5, "should return 5 remaining boons")
end

-- Modifier tests

function test_double_damage()
    local config = modifiers.apply("double_damage")
    assert(config.damage_mult == 2, "double damage should set mult to 2")
end

function test_small_arena()
    local config = modifiers.apply("small_arena")
    assert(config.arena_w == 600, "small arena width should be 600")
    assert(config.arena_h == 400, "small arena height should be 400")
end

function test_no_modifier()
    local config = modifiers.apply(nil)
    assert(not config.damage_mult, "nil modifier should not set damage_mult")
end

-- Match tests

function test_init()
    local state = match_env.init({})
    assert(state.players, "should have players table")
    assert(state.phase == "playing", "should start in playing phase")
    assert(state.round == 1, "should start at round 1")
    assert(state.arena_w == 800, "default arena width")
    assert(state.arena_h == 600, "default arena height")
end

function test_join_and_leave()
    local state = match_env.init({})
    state = match_env.join("p1", state)
    assert(state.players.p1, "player should exist")
    assert(state.players.p1.hp == 100, "player should have 100 hp")

    state = match_env.leave("p1", state)
    assert(not state.players.p1, "player should be removed")
end

function test_movement_clamped()
    local state = match_env.init({})
    state = match_env.join("p1", state)
    -- Move far left
    for i = 1, 300 do
        state = match_env.handle_input("p1", {left = true}, state)
    end
    assert(state.players.p1.x >= 16, "x should be clamped to player radius")
end

function test_shooting_creates_projectile()
    local state = match_env.init({})
    state = match_env.join("p1", state)
    local px = state.players.p1.x
    local py = state.players.p1.y
    state = match_env.handle_input("p1", {
        shoot = true, aim_x = px + 100, aim_y = py
    }, state)
    assert(#state.projectiles == 1, "should create one projectile")
    assert(state.projectiles[1].owner == "p1", "projectile should be owned by p1")
end

function test_shooting_respects_cooldown()
    local state = match_env.init({})
    state = match_env.join("p1", state)
    local px = state.players.p1.x
    local py = state.players.p1.y
    -- First shot
    state = match_env.handle_input("p1", {shoot = true, aim_x = px + 100, aim_y = py}, state)
    -- Second shot immediately (should fail due to cooldown)
    state = match_env.handle_input("p1", {shoot = true, aim_x = px + 100, aim_y = py}, state)
    assert(#state.projectiles == 1, "cooldown should prevent second shot")
end

function test_tick_increments()
    local state = match_env.init({})
    state = match_env.tick(state)
    assert(state.tick_count == 1, "tick_count should increment")
end

function test_get_state_playing()
    local state = match_env.init({})
    state = match_env.join("p1", state)
    local view = match_env.get_state("p1", state)
    assert(view.phase == "playing", "should show playing phase")
    assert(view.players, "should include players")
    assert(view.arena_w == 800, "should include arena width")
    assert(view.time_remaining, "should include time_remaining")
end
