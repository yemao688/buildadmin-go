package permissioncache

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheLoadsGroupsOnceAndCopiesValues(t *testing.T) {
	cache := New[int, string]()
	loads := 0
	loader := func() ([]int, error) {
		loads++
		return []int{1, 2}, nil
	}

	groups, err := cache.GetOrLoadGroups(7, loader)
	require.NoError(t, err)
	groups[0] = 99
	hit, err := cache.GetOrLoadGroups(7, loader)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, hit)
	require.Equal(t, 1, loads)
}

func TestCacheReloadClonesOnWriteAndReadPathsReturnInternalStorage(t *testing.T) {
	cache := New[int, string]()
	rules := []string{"dashboard"}
	names := []string{"dashboard/index"}
	returnedNames, err := cache.ReloadRules(7, func() ([]string, []string, error) {
		return rules, names, nil
	})
	require.NoError(t, err)
	// The reload return value is a defensive copy (write path): mutating it
	// must not corrupt the cached state.
	returnedNames[0] = "changed"
	require.Equal(t, []string{"dashboard/index"}, cache.RuleNames(7))

	// Read paths return the cache's internal backing storage (zero-copy) and
	// pin the read-only contract: mutations are visible through the cache, so
	// callers must treat read results as immutable. Re-introducing a defensive
	// copy on the read path would make this assertion fail.
	readRules, ok := cache.Rules(7)
	require.True(t, ok)
	readRules[0] = "mutated"
	require.Equal(t, []string{"mutated"}, cacheRules(cache, 7))
	require.Equal(t, []string{"dashboard/index"}, cache.RuleNames(7))
}

func TestCacheRuleNamesSetMatchesRuleNamesMembership(t *testing.T) {
	cache := New[int, string]()
	_, err := cache.ReloadRules(7, func() ([]string, []string, error) {
		return []string{"rule"}, []string{"dashboard/index", "auth/login", "dashboard/index"}, nil
	})
	require.NoError(t, err)

	names := cache.RuleNames(7)
	set := cache.RuleNamesSet(7)
	require.Len(t, set, 2) // duplicates in the loader output collapse in the set
	for _, n := range names {
		_, ok := set[n]
		require.True(t, ok, "set must contain every element of RuleNames: %q", n)
	}
	// Unknown names must be absent from both.
	for _, n := range []string{"", "auth/login/extra", "Auth/Login"} {
		require.False(t, slices.Contains(names, n), "slice must not contain %q", n)
		_, ok := set[n]
		require.False(t, ok, "set must not contain %q", n)
	}

	// Invalidation clears the per-user set along with the slice.
	cache.InvalidateUser(7)
	require.Nil(t, cache.RuleNamesSet(7))
}

func TestCacheAllNamesSetMatchesAllRuleNamesMembership(t *testing.T) {
	cache := New[int, string]()
	loads := 0
	loader := func() ([]string, error) {
		loads++
		return []string{"auth/login", "dashboard/index", "auth/login"}, nil
	}

	names, err := cache.AllRuleNames(loader)
	require.NoError(t, err)
	require.Equal(t, 1, loads)
	// The set shares the same single load and the same stored names.
	set, err := cache.AllNamesSet(loader)
	require.NoError(t, err)
	require.Equal(t, 1, loads)
	require.Len(t, set, 2)
	for _, n := range names {
		_, ok := set[n]
		require.True(t, ok, "set must contain every element of AllRuleNames: %q", n)
	}
	_, ok := set["auth/login/extra"]
	require.False(t, ok)

	// Invalidation clears both and the next load rebuilds them consistently.
	cache.InvalidateAll()
	_, err = cache.AllRuleNames(loader)
	require.NoError(t, err)
	require.Equal(t, 2, loads)
	set, err = cache.AllNamesSet(loader)
	require.NoError(t, err)
	require.Equal(t, 2, loads)
	require.Len(t, set, 2)
}

func TestCacheAllNamesSetPropagatesLoaderError(t *testing.T) {
	cache := New[int, string]()
	_, err := cache.AllRuleNames(func() ([]string, error) { return nil, errors.New("boom") })
	require.Error(t, err)
	_, err = cache.AllNamesSet(func() ([]string, error) { return nil, errors.New("boom") })
	require.Error(t, err)
}

func TestCacheInvalidationClearsGroupsRulesAndNames(t *testing.T) {
	cache := New[int, string]()
	loadGroups := func() ([]int, error) { return []int{1}, nil }
	loadRules := func() ([]string, []string, error) {
		return []string{"rule"}, []string{"rule/name"}, nil
	}
	_, err := cache.GetOrLoadGroups(1, loadGroups)
	require.NoError(t, err)
	_, err = cache.ReloadRules(1, loadRules)
	require.NoError(t, err)
	cache.InvalidateUser(1)
	_, ok := cache.Rules(1)
	require.False(t, ok)
	require.Empty(t, cache.RuleNames(1))
	groups, err := cache.GetOrLoadGroups(1, loadGroups)
	require.NoError(t, err)
	require.Equal(t, []int{1}, groups)

	_, err = cache.ReloadRules(2, loadRules)
	require.NoError(t, err)
	cache.InvalidateAll()
	_, ok = cache.Rules(2)
	require.False(t, ok)
}

func TestCacheReloadKeepsRuleThenGroupLockOrder(t *testing.T) {
	cache := New[int, string]()
	names, err := cache.ReloadRules(1, func() ([]string, []string, error) {
		groups, err := cache.GetOrLoadGroups(1, func() ([]int, error) { return []int{1}, nil })
		if err != nil {
			return nil, nil, err
		}
		return []string{"rule"}, []string{"group-" + string(rune('0'+groups[0]))}, nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"group-1"}, names)
}

func TestCacheConcurrentReadsReloadsAndInvalidation(t *testing.T) {
	cache := New[int, string]()
	var groupLoads atomic.Int32
	var ruleLoads atomic.Int32
	groupLoader := func() ([]int, error) {
		groupLoads.Add(1)
		return []int{1, 2}, nil
	}
	ruleLoader := func() ([]string, []string, error) {
		ruleLoads.Add(1)
		return []string{"rule"}, []string{"rule/name"}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = cache.GetOrLoadGroups(1, groupLoader)
				_, _ = cache.ReloadRules(1, ruleLoader)
				_, _ = cache.Rules(1)
				_ = cache.RuleNames(1)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			cache.InvalidateUser(1)
			cache.InvalidateAll()
		}
	}()
	wg.Wait()
	require.Positive(t, groupLoads.Load())
	require.Positive(t, ruleLoads.Load())
}

func cacheRules(cache *Cache[int, string], uid int32) []string {
	rules, _ := cache.Rules(uid)
	return rules
}
