-- KEYS[1] = bucket key (tenant:route)
-- ARGV[1] = max_tokens
-- ARGV[2] = refill_rate_per_sec
-- ARGV[3] = now (ms)
-- ARGV[4] = requested_tokens (usually 1)

local bucket = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(bucket[1]) or tonumber(ARGV[1])
local ts = tonumber(bucket[2]) or tonumber(ARGV[3])

local elapsed = math.max(0, tonumber(ARGV[3]) - ts) / 1000
local refill = elapsed * tonumber(ARGV[2])
tokens = math.min(tonumber(ARGV[1]), tokens + refill)

local allowed = 0
if tokens >= tonumber(ARGV[4]) then
  tokens = tokens - tonumber(ARGV[4])
  allowed = 1
end

redis.call('HMSET', KEYS[1], 'tokens', tokens, 'ts', ARGV[3])
redis.call('EXPIRE', KEYS[1], 3600)

return {allowed, math.floor(tokens)}
