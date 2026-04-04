-- Round modifier system for arena matches
local M = {}

M.all = {
    { id = "double_damage", name = "Double Damage", description = "All damage is doubled" },
    { id = "small_arena", name = "Tight Quarters", description = "Arena shrinks to 600x400" },
    { id = "fast_projectiles", name = "Bullet Storm", description = "Projectiles move 2x faster" },
    { id = "low_hp", name = "Glass Cannon", description = "Everyone starts at 50 HP" },
    { id = "extra_lives", name = "Second Chance", description = "3 respawns per player" },
    { id = "rapid_fire", name = "Trigger Happy", description = "Cooldown halved for everyone" }
}

function M.vote_options()
    local opts = {}
    for _, mod in ipairs(M.all) do
        table.insert(opts, { id = mod.id, label = mod.name })
    end
    return opts
end

function M.apply(modifier_id)
    local config = {}

    if modifier_id == "double_damage" then
        config.damage_mult = 2
    elseif modifier_id == "small_arena" then
        config.arena_w = 600
        config.arena_h = 400
    elseif modifier_id == "fast_projectiles" then
        config.projectile_speed_mult = 2
    elseif modifier_id == "low_hp" then
        config.hp_override = 50
    elseif modifier_id == "extra_lives" then
        config.lives = 3
    elseif modifier_id == "rapid_fire" then
        config.cooldown_mult = 0.5
    end

    return config
end

return M
