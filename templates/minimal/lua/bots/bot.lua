-- Bot AI script
-- Called every tick for each bot player

names = {"Spark", "Blitz", "Volt", "Neon"}

function think(bot_id, state)
    return {
        right = math.random(2) == 1,
        left = math.random(2) == 1,
        up = math.random(2) == 1,
        down = math.random(2) == 1
    }
end
