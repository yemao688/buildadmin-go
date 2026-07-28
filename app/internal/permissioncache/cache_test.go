package permissioncache

import (
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

func TestCacheReloadsRulesAndCopiesReadValues(t *testing.T) {
	cache := New[int, string]()
	rules := []string{"dashboard"}
	names := []string{"dashboard/index"}
	returnedNames, err := cache.ReloadRules(7, func() ([]string, []string, error) {
		return rules, names, nil
	})
	require.NoError(t, err)
	returnedNames[0] = "changed"

	readRules, ok := cache.Rules(7)
	require.True(t, ok)
	readRules[0] = "changed"
	require.Equal(t, []string{"dashboard"}, cacheRules(cache, 7))
	require.Equal(t, []string{"dashboard/index"}, cache.RuleNames(7))
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
