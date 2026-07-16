-- 验证验证码是否存在
local key = KEYS[1]
local ctnKey = 'ctn'
local ttl = redis.call("HTTL", key, 'FIELDS', 1, 'code')

-- ttl = -1说明验证码存在但是没有过期时间
if ttl[1] == -1 then
    -- 有人误操作
    return -2

    -- ttl = -2说明验证码不存在，或者过期时间已经超过1分钟
    -- ttl = 600-60 剩余有效时间
elseif ttl[1] == -2 or ttl[1] < 240 then
    -- 设置验证码
    local code = ARGV[1]
    redis.call("HMSET", key, 'code', code, ctnKey, 3)
    redis.call("HEXPIRE", key, 300,'FIELDS', 2, 'code', ctnKey)
    return 0
else
    -- 操作频繁
    return -1
end
