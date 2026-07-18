-- match.lua - a real-time chat room on Asobi.
-- Everyone who joins shares one room. A {say = "..."} input is broadcast to all
-- connected players as a "chat" event. The author is taken from player_id, so a
-- client can't forge who said what. See
-- https://github.com/widgrensit/asobi_lua for the full game.* API.

match_size = 2
max_players = 50

function init(config)
    return { players = {} }
end

function join(player_id, state)
    state.players[player_id] = true
    game.broadcast("chat", { system = true, text = player_id .. " joined" })
    return state
end

function leave(player_id, state)
    state.players[player_id] = nil
    game.broadcast("chat", { system = true, text = player_id .. " left" })
    return state
end

-- handle_input turns a {say = "..."} message into a broadcast to the room.
function handle_input(player_id, input, state)
    if input and type(input.say) == "string" and #input.say > 0 then
        game.broadcast("chat", { from = player_id, text = input.say })
    end
    return state
end

function tick(state)
    return state
end

function get_state(player_id, state)
    return state
end
