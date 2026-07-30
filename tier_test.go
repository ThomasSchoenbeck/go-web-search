package main

import (
	"testing"
	"time"
)

func testTierConfig() TierConfig {
	return TierConfig{
		ShortTTL:         Duration{10 * 24 * time.Hour},
		LongTTL:          Duration{45 * 24 * time.Hour},
		PromoteAfterHits: 3,
	}
}

func TestPromote(t *testing.T) {
	c := testTierConfig()
	cases := []struct {
		name string
		tier string
		hits int
		want string
	}{
		{"short below threshold stays short", tierShort, 2, tierShort},
		{"short at threshold promotes to long", tierShort, 3, tierLong},
		{"short above threshold promotes to long", tierShort, 10, tierLong},
		{"long stays long", tierLong, 100, tierLong},
		{"permanent stays permanent", tierPermanent, 0, tierPermanent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := promote(tc.tier, tc.hits, c); got != tc.want {
				t.Fatalf("promote(%q, %d) = %q, want %q", tc.tier, tc.hits, got, tc.want)
			}
		})
	}
}

func TestNextExpiry(t *testing.T) {
	c := testTierConfig()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("short slides to now+short_ttl", func(t *testing.T) {
		tier, exp, perm := nextExpiry(now, tierShort, 0, c)
		if perm || tier != tierShort {
			t.Fatalf("tier=%q perm=%v", tier, perm)
		}
		if want := now.Add(10 * 24 * time.Hour); !exp.Equal(want) {
			t.Fatalf("expiry=%v want %v", exp, want)
		}
	})

	t.Run("promotion uses the long ttl", func(t *testing.T) {
		tier, exp, perm := nextExpiry(now, tierShort, 3, c)
		if perm || tier != tierLong {
			t.Fatalf("tier=%q perm=%v", tier, perm)
		}
		if want := now.Add(45 * 24 * time.Hour); !exp.Equal(want) {
			t.Fatalf("expiry=%v want %v", exp, want)
		}
	})

	t.Run("permanent has no expiry", func(t *testing.T) {
		tier, exp, perm := nextExpiry(now, tierPermanent, 0, c)
		if !perm || tier != tierPermanent {
			t.Fatalf("tier=%q perm=%v", tier, perm)
		}
		if !exp.IsZero() {
			t.Fatalf("expiry=%v want zero", exp)
		}
	})
}

func TestNormalizeTier(t *testing.T) {
	cases := []struct {
		v, def, want string
	}{
		{"", "short", tierShort},
		{"", "long", tierLong},
		{"bogus", "long", tierLong},
		{"permanent", "short", tierPermanent},
		{"bogus", "alsobogus", tierShort},
	}
	for _, tc := range cases {
		if got := normalizeTier(tc.v, tc.def); got != tc.want {
			t.Fatalf("normalizeTier(%q, %q) = %q, want %q", tc.v, tc.def, got, tc.want)
		}
	}
}
