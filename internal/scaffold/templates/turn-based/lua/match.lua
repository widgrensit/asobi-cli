-- match.lua - a turn-based game on Asobi.
-- Two players alternate claiming cells on a 3x3 board. The server owns whose
-- turn it is and which cells are taken, so an out-of-turn or duplicate move is
-- rejected server-side rather than trusted from the client. See
-- https://github.com/widgrensit/asobi_lua for the full game.* API.

match_size = 2

function init(config)
    return { order = {}, turn = 1, board = {} }
end

function join(player_id, state)
    table.insert(state.order, player_id)
    return state
end

function leave(player_id, state)
    return state
end

-- handle_input claims a cell for the current player. It ignores the move unless
-- it is this player's turn and the cell (1-9) is free, then passes the turn.
function handle_input(player_id, input, state)
    if player_id ~= state.order[state.turn] or not input then
        return state
    end

    local cell = input.cell
    if type(cell) ~= "number" or cell < 1 or cell > 9 or state.board[cell] then
        return state
    end

    state.board[cell] = player_id
    state.turn = (state.turn % #state.order) + 1
    game.broadcast("move", {
        by = player_id,
        cell = cell,
        next = state.order[state.turn]
    })
    return state
end

function tick(state)
    return state
end

function get_state(player_id, state)
    return state
end
