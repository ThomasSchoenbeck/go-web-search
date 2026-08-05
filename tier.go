package main

import "time"

// Tier names for cache and memory rows. A permanent row never expires.
const (
	tierShort     = "short"
	tierLong      = "long"
	tierPermanent = "permanent"
)

// tierTTL returns the sliding-window lifetime for a tier and whether the tier is
// permanent (no expiry). An unknown tier is treated as short.
func tierTTL(tier string, c TierConfig) (ttl time.Duration, permanent bool) {
	switch tier {
	case tierPermanent:
		return 0, true
	case tierLong:
		return c.LongTTL.Duration, false
	default:
		return c.ShortTTL.Duration, false
	}
}

// promote raises a tier based on hit count. Automatic promotion stops at
// "long": "permanent" is only ever set deliberately via the remember flag, so a
// frequently used row is kept a long time without silently becoming
// un-collectable.
func promote(tier string, hits int, c TierConfig) string {
	if tier == tierPermanent {
		return tierPermanent
	}
	if tier == tierShort && hits >= c.PromoteAfterHits {
		return tierLong
	}
	return tier
}

// nextExpiry computes the tier and expiry for a row after a store or a hit. The
// window slides: expiry is always now + the (possibly promoted) tier's TTL. A
// permanent row returns a zero time and permanent=true, meaning "no expiry".
func nextExpiry(now time.Time, tier string, hits int, c TierConfig) (newTier string, expires time.Time, permanent bool) {
	newTier = promote(tier, hits, c)
	ttl, perm := tierTTL(newTier, c)
	if perm {
		return newTier, time.Time{}, true
	}
	return newTier, now.Add(ttl), false
}

// normalizeTier maps a caller-supplied remember value to a valid tier, falling
// back to the configured default and then to short.
func normalizeTier(v, def string) string {
	switch v {
	case tierShort, tierLong, tierPermanent:
		return v
	}
	switch def {
	case tierShort, tierLong, tierPermanent:
		return def
	}
	return tierShort
}
