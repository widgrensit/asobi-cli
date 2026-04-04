local M = {}

local function exec(cmd)
    return os.execute(cmd)
end

function M.dev()
    print("Starting game server...")
    exec("docker compose up -d")
    print("")
    print("Game server running. Connect your client.")
    print("  asobi logs      # tail server logs")
    print("  asobi state     # show active matches")
    print("  asobi stop      # stop the server")
end

function M.stop()
    exec("docker compose down")
end

function M.logs()
    exec("docker compose logs -f asobi")
end

function M.state()
    local handle = io.popen(
        'docker compose exec -T asobi bin/asobi eval \'' ..
        'Groups = pg:which_groups(nova_scope),' ..
        'Matches = [{T,I} || {T,I} = {asobi_match_server, _} <- Groups],' ..
        'lists:foreach(fun({_, MId}) -> ' ..
        '  [Pid|_] = pg:get_members(nova_scope, {asobi_match_server, MId}),' ..
        '  Info = asobi_match_server:get_info(Pid),' ..
        '  io:format("~s  players: ~p  status: ~p~n", [MId, maps:get(player_count, Info), maps:get(status, Info)])' ..
        ' end, Matches),' ..
        'case Matches of [] -> io:format("No active matches~n"); _ -> ok end.' ..
        "' 2>/dev/null"
    )
    if handle then
        for line in handle:lines() do
            print(line)
        end
        handle:close()
    else
        print("Error: could not connect to game server. Is it running?")
    end
end

return M
