-- Tests for match.lua
-- Run with: asobi test

package.path = "../lua/?.lua;" .. package.path
local match_env = setmetatable({}, {__index = _G})
local chunk = assert(loadfile("lua/match.lua", "t", match_env))
chunk()

function test_init()
    local state = match_env.init({})
    assert(state.players, "should have players table")
    assert(state.tick_count == 0, "tick_count should start at 0")
end

function test_join()
    local state = match_env.init({})
    state = match_env.join("player1", state)
    assert(state.players.player1, "player should exist after join")
    assert(state.players.player1.hp == 100, "player should have 100 hp")
end

function test_leave()
    local state = match_env.init({})
    state = match_env.join("player1", state)
    state = match_env.leave("player1", state)
    assert(not state.players.player1, "player should be removed after leave")
end

function test_movement()
    local state = match_env.init({})
    state = match_env.join("player1", state)
    local start_x = state.players.player1.x
    state = match_env.handle_input("player1", {right = true}, state)
    assert(state.players.player1.x > start_x, "player should move right")
end

function test_tick_increments()
    local state = match_env.init({})
    state = match_env.tick(state)
    assert(state.tick_count == 1, "tick_count should increment")
    state = match_env.tick(state)
    assert(state.tick_count == 2, "tick_count should increment again")
end

function test_match_finishes()
    local state = match_env.init({})
    state.tick_count = 899
    state = match_env.tick(state)
    assert(state._finished == true, "match should finish at 900 ticks")
    assert(state._result.status == "completed", "result should be completed")
end
