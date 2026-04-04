-- Boon (power-up) system for arena matches
local M = {}

M.all = {
    { id = "hp_boost", name = "Vitality", description = "+15 Max HP", stat = "max_hp", delta = 15 },
    { id = "damage_boost", name = "Power Shot", description = "+8 Damage", stat = "damage", delta = 8 },
    { id = "fast_projectiles", name = "Velocity", description = "+3 Projectile Speed", stat = "projectile_speed", delta = 3 },
    { id = "short_cooldown", name = "Rapid Fire", description = "-4 Shot Cooldown", stat = "shoot_cooldown", delta = -4 },
    { id = "large_projectiles", name = "Wide Bore", description = "+3 Projectile Radius", stat = "projectile_radius", delta = 3 },
    { id = "lifesteal", name = "Vampiric", description = "+15% Lifesteal", stat = "lifesteal", delta = 15 },
    { id = "speed_boost", name = "Swift", description = "+1 Movement Speed", stat = "speed", delta = 1 }
}

function M.default_stats()
    return {
        max_hp = 100,
        damage = 25,
        speed = 4,
        projectile_speed = 8,
        shoot_cooldown = 15,
        projectile_radius = 4,
        lifesteal = 0,
        boons = {}
    }
end

function M.find(boon_id)
    for _, boon in ipairs(M.all) do
        if boon.id == boon_id then
            return boon
        end
    end
    return nil
end

function M.apply(boon_id, stats)
    local boon = M.find(boon_id)
    if not boon then return stats end

    local current = stats[boon.stat] or 0
    local new_val = current + boon.delta

    if boon.stat == "shoot_cooldown" then
        new_val = math.max(5, new_val)
    else
        new_val = math.max(0, new_val)
    end

    stats[boon.stat] = new_val

    if not stats.boons then stats.boons = {} end
    table.insert(stats.boons, boon_id)

    return stats
end

function M.random_choices(n, already_picked)
    local available = {}
    for _, boon in ipairs(M.all) do
        local picked = false
        for _, id in ipairs(already_picked or {}) do
            if id == boon.id then
                picked = true
                break
            end
        end
        if not picked then
            table.insert(available, boon)
        end
    end

    local choices = {}
    for i = 1, n do
        if #available == 0 then break end
        local idx = math.random(#available)
        table.insert(choices, available[idx])
        table.remove(available, idx)
    end
    return choices
end

return M
