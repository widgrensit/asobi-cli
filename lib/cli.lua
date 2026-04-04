local M = {}

local template = require("template")
local docker = require("docker")
local test = require("test")

local commands = {}

commands.new = {
    usage = "asobi new <template> <name>",
    desc = "Scaffold a new game project",
    run = function(cli_root, args)
        local tmpl = args[1]
        local name = args[2]
        if not tmpl or not name then
            print("Usage: asobi new <template> <name>")
            print("")
            print("Available templates:")
            template.list(cli_root)
            os.exit(1)
        end
        template.create(cli_root, tmpl, name)
    end
}

commands.dev = {
    usage = "asobi dev",
    desc = "Start the game server (docker compose up + logs)",
    run = function(_, _)
        docker.dev()
    end
}

commands.stop = {
    usage = "asobi stop",
    desc = "Stop the game server",
    run = function(_, _)
        docker.stop()
    end
}

commands.test = {
    usage = "asobi test [file]",
    desc = "Run Lua tests",
    run = function(_, args)
        local file = args[1]
        test.run(file)
    end
}

commands.state = {
    usage = "asobi state",
    desc = "Show active matches and player counts",
    run = function(_, _)
        docker.state()
    end
}

commands.logs = {
    usage = "asobi logs",
    desc = "Tail game server logs",
    run = function(_, _)
        docker.logs()
    end
}

commands.help = {
    usage = "asobi help",
    desc = "Show this help message",
    run = function(_, _)
        M.help()
    end
}

function M.help()
    print("asobi - game development CLI")
    print("")
    print("Commands:")
    local order = {"new", "dev", "stop", "test", "state", "logs", "help"}
    for _, name in ipairs(order) do
        local cmd = commands[name]
        print(string.format("  %-30s %s", cmd.usage, cmd.desc))
    end
end

function M.run(cli_root, args)
    local cmd_name = args[1]

    if not cmd_name or cmd_name == "-h" or cmd_name == "--help" then
        M.help()
        return
    end

    local cmd = commands[cmd_name]
    if not cmd then
        print("Unknown command: " .. cmd_name)
        print("")
        M.help()
        os.exit(1)
    end

    -- Shift args: remove the command name
    local cmd_args = {}
    for i = 2, #args do
        cmd_args[#cmd_args + 1] = args[i]
    end

    cmd.run(cli_root, cmd_args)
end

return M
