local key = KEYS[1]
local expectedCode = ARGV[1]
local cache = redis.call("HMGET", key, "code", "ctn")


local ctn = tonumber(cache[2])
local code = cache[1]
-- 验证次数耗尽
if ctn == nil or ctn <= 0 then
    return -1
end
--验证码相等
if expectedCode == code then
    -- 相等也不能删除验证码
    redis.call("HSET", key, "ctn", -1)
    return 0
else
    -- 可能输错减少验证次数
    redis.call("HINCRBY", key, "ctn", -1)
    return -2
end
