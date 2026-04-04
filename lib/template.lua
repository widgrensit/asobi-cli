local M = {}

local function dir_exists(path)
    local f = io.open(path, "r")
    if f then f:close() return true end
    return os.execute('test -d "' .. path .. '"') == true
end

local function copy_dir(src, dst)
    os.execute('cp -r "' .. src .. '" "' .. dst .. '"')
end

local function replace_in_file(path, pattern, replacement)
    local f = io.open(path, "r")
    if not f then return end
    local content = f:read("*a")
    f:close()
    content = content:gsub(pattern, replacement)
    f = io.open(path, "w")
    f:write(content)
    f:close()
end

function M.list(cli_root)
    local templates_dir = cli_root .. "templates/"
    local handle = io.popen('ls -1 "' .. templates_dir .. '"')
    if not handle then return end
    for line in handle:lines() do
        local readme = templates_dir .. line .. "/README.md"
        local f = io.open(readme, "r")
        local desc = ""
        if f then
            desc = f:read("*l") or ""
            desc = desc:gsub("^#+ *", "")
            f:close()
        end
        print(string.format("  %-15s %s", line, desc))
    end
    handle:close()
end

function M.create(cli_root, tmpl_name, project_name)
    local tmpl_dir = cli_root .. "templates/" .. tmpl_name

    if not dir_exists(tmpl_dir) then
        print("Error: template '" .. tmpl_name .. "' not found")
        print("")
        print("Available templates:")
        M.list(cli_root)
        os.exit(1)
    end

    if dir_exists(project_name) then
        print("Error: directory '" .. project_name .. "' already exists")
        os.exit(1)
    end

    copy_dir(tmpl_dir, project_name)

    -- Replace placeholders
    local db_name = project_name:gsub("-", "_") .. "_dev"
    replace_in_file(project_name .. "/docker-compose.yml", "{{PROJECT_NAME}}", project_name)
    replace_in_file(project_name .. "/docker-compose.yml", "{{DB_NAME}}", db_name)
    replace_in_file(project_name .. "/README.md", "{{PROJECT_NAME}}", project_name)

    print("Created " .. project_name .. "/ from '" .. tmpl_name .. "' template")
    print("")
    print("Next steps:")
    print("  cd " .. project_name)
    print("  asobi dev       # start the game server")
    print("  asobi test      # run tests")
end

return M
