local M = {}

local RED = "\27[31m"
local GREEN = "\27[32m"
local RESET = "\27[0m"

local function find_test_files(dir)
    local files = {}
    local handle = io.popen('find "' .. dir .. '" -name "*_test.lua" -type f 2>/dev/null | sort')
    if not handle then return files end
    for line in handle:lines() do
        files[#files + 1] = line
    end
    handle:close()
    return files
end

local function discover_tests(env)
    local tests = {}
    for k, v in pairs(env) do
        if type(v) == "function" and type(k) == "string" and k:sub(1, 5) == "test_" then
            tests[#tests + 1] = k
        end
    end
    table.sort(tests)
    return tests
end

local function run_file(path, game_lua_dir)
    local filename = path:match("([^/]+)$")
    print(filename)

    -- Set package.path so test files can require game modules
    local old_path = package.path
    package.path = game_lua_dir .. "/?.lua;" .. game_lua_dir .. "/?/init.lua;" .. package.path

    -- Load the test file in a fresh environment with access to globals
    local env = setmetatable({}, {__index = _G})
    local chunk, err = loadfile(path, "t", env)
    if not chunk then
        print("  " .. RED .. "LOAD ERROR: " .. err .. RESET)
        package.path = old_path
        return 0, 1
    end

    local ok, load_err = pcall(chunk)
    if not ok then
        print("  " .. RED .. "LOAD ERROR: " .. tostring(load_err) .. RESET)
        package.path = old_path
        return 0, 1
    end

    local tests = discover_tests(env)
    if #tests == 0 then
        print("  (no test_ functions found)")
        package.path = old_path
        return 0, 0
    end

    local passed, failed = 0, 0
    for _, name in ipairs(tests) do
        local test_ok, test_err = pcall(env[name])
        if test_ok then
            print("  " .. GREEN .. "pass" .. RESET .. "  " .. name)
            passed = passed + 1
        else
            local msg = tostring(test_err):match(":[%d]+: (.+)") or tostring(test_err)
            print("  " .. RED .. "FAIL" .. RESET .. "  " .. name .. ": " .. msg)
            failed = failed + 1
        end
    end

    package.path = old_path
    return passed, failed
end

function M.run(file)
    -- Find the lua/ directory (game modules) and test/ directory
    local game_lua_dir = "lua"
    local test_dir = "test"

    if not io.open(game_lua_dir, "r") and not os.execute('test -d "' .. game_lua_dir .. '"') then
        print("Error: no lua/ directory found. Are you in a game project?")
        os.exit(1)
    end

    -- Resolve absolute path for require()
    local abs_handle = io.popen('cd "' .. game_lua_dir .. '" && pwd')
    if abs_handle then
        game_lua_dir = abs_handle:read("*l") or game_lua_dir
        abs_handle:close()
    end

    local files
    if file then
        files = {file}
    else
        files = find_test_files(test_dir)
    end

    if #files == 0 then
        print("No test files found in " .. test_dir .. "/")
        print("Test files should be named *_test.lua")
        os.exit(1)
    end

    print("")
    local total_passed, total_failed = 0, 0
    for _, f in ipairs(files) do
        local p, fl = run_file(f, game_lua_dir)
        total_passed = total_passed + p
        total_failed = total_failed + fl
        print("")
    end

    local total = total_passed + total_failed
    if total_failed > 0 then
        print(string.format("%d tests, %s%d passed%s, %s%d failed%s",
            total, GREEN, total_passed, RESET, RED, total_failed, RESET))
        os.exit(1)
    else
        print(string.format("%s%d tests, all passed%s", GREEN, total, RESET))
    end
end

return M
