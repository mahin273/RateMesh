-- KEYS[1] = rate limit key (sorted set)
-- ARGV[1] = now (ms)
-- ARGV[2] = window (ms)
-- ARGV[3] = limit
-- ARGV[4] = request_id (unique value)

local clear_before = tonumber(ARGV[1]) - tonumber(ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', clear_before)

local current_requests = redis.call('ZCARD', KEYS[1])
local allowed = 0

if current_requests < tonumber(ARGV[3]) then
  redis.call('ZADD', KEYS[1], ARGV[1], ARGV[4])
  redis.call('EXPIRE', KEYS[1], math.ceil(tonumber(ARGV[2]) / 1000))
  allowed = 1
  current_requests = current_requests + 1
end

return {allowed, tonumber(ARGV[3]) - current_requests}
