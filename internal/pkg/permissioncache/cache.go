// Package permissioncache contains the private, shared cache mechanics used
// by the admin and frontend permission models.
package permissioncache

import "sync"

// Cache stores group membership and the derived permission rules for each
// user. Rule reloads hold the rule lock while loading groups, so callers must
// keep the rule-then-group lock order when adding cache operations.
//
// Read paths return the cache's internal storage (slices and maps) without
// copying: the returned values are read-only and callers must never modify
// them. This is safe under concurrency because writers always replace stored
// values wholesale (clone on write) and never mutate previously published
// slices or maps in place; readers hold a reference to an immutable snapshot.
type Cache[G any, R any] struct {
	groupMu      sync.RWMutex
	groupList    map[int32][]G
	ruleMu       sync.RWMutex
	ruleList     map[int32][]R
	ruleNames    map[int32][]string
	ruleNameSets map[int32]map[string]struct{}
	allNames     []string
	allNamesSet  map[string]struct{}
	allNamesOK   bool
}

func New[G any, R any]() *Cache[G, R] {
	return &Cache[G, R]{
		groupList:    make(map[int32][]G),
		ruleList:     make(map[int32][]R),
		ruleNames:    make(map[int32][]string),
		ruleNameSets: make(map[int32]map[string]struct{}),
	}
}

// GetOrLoadGroups returns a read-only copy of the cached groups or loads them
// once while holding the group write lock. The loader result is cached even
// when it also returns an error, matching the existing model behavior.
func (c *Cache[G, R]) GetOrLoadGroups(uid int32, loader func() ([]G, error)) ([]G, error) {
	c.groupMu.RLock()
	if groups, ok := c.groupList[uid]; ok {
		copyOfGroups := clone(groups)
		c.groupMu.RUnlock()
		return copyOfGroups, nil
	}
	c.groupMu.RUnlock()

	c.groupMu.Lock()
	defer c.groupMu.Unlock()
	if groups, ok := c.groupList[uid]; ok {
		return clone(groups), nil
	}
	groups, err := loader()
	if c.groupList == nil {
		c.groupList = make(map[int32][]G)
	}
	c.groupList[uid] = clone(groups)
	return clone(groups), err
}

// ReloadRules reloads and stores a user's rules and rule names. The loader is
// called while the rule write lock is held; it may load groups through this
// cache, preserving the rule-then-group lock order.
func (c *Cache[G, R]) ReloadRules(uid int32, loader func() ([]R, []string, error)) ([]string, error) {
	c.ruleMu.Lock()
	defer c.ruleMu.Unlock()

	rules, names, err := loader()
	if err != nil {
		return nil, err
	}
	if c.ruleList == nil {
		c.ruleList = make(map[int32][]R)
	}
	if c.ruleNames == nil {
		c.ruleNames = make(map[int32][]string)
	}
	if c.ruleNameSets == nil {
		c.ruleNameSets = make(map[int32]map[string]struct{})
	}
	c.ruleList[uid] = clone(rules)
	c.ruleNames[uid] = clone(names)
	c.ruleNameSets[uid] = nameSet(names)
	return clone(names), nil
}

// Rules returns the cached rules for uid. The returned slice is the cache's
// internal storage: it is read-only and callers must not modify it (reloads
// replace the stored slice wholesale, never mutating it in place).
func (c *Cache[G, R]) Rules(uid int32) ([]R, bool) {
	c.ruleMu.RLock()
	defer c.ruleMu.RUnlock()
	rules, ok := c.ruleList[uid]
	return rules, ok
}

// RuleNames returns the cached rule names for uid. The returned slice is the
// cache's internal storage: it is read-only and callers must not modify it.
func (c *Cache[G, R]) RuleNames(uid int32) []string {
	c.ruleMu.RLock()
	defer c.ruleMu.RUnlock()
	return c.ruleNames[uid]
}

// RuleNamesSet returns the cached rule names for uid as a membership set. The
// returned map is the cache's internal storage: it is read-only and callers
// must not modify it. The set is built from the very same normalized names as
// RuleNames, so membership matches slices.Contains over RuleNames exactly.
func (c *Cache[G, R]) RuleNamesSet(uid int32) map[string]struct{} {
	c.ruleMu.RLock()
	defer c.ruleMu.RUnlock()
	return c.ruleNameSets[uid]
}

// AllRuleNames returns the cached names for every rule, loading them once
// when the cache is cold. The returned slice is the cache's internal storage:
// it is read-only and callers must not modify it.
func (c *Cache[G, R]) AllRuleNames(loader func() ([]string, error)) ([]string, error) {
	names, _, err := c.loadAllNames(loader)
	return names, err
}

// AllNamesSet returns the cached names for every rule as a membership set,
// loading them once when the cache is cold. The returned map is the cache's
// internal storage: it is read-only and callers must not modify it. The set
// is built from the very same normalized names as AllRuleNames, so membership
// matches slices.Contains over AllRuleNames exactly.
func (c *Cache[G, R]) AllNamesSet(loader func() ([]string, error)) (map[string]struct{}, error) {
	_, set, err := c.loadAllNames(loader)
	return set, err
}

// loadAllNames returns the internal global rule-name slice and set, loading
// them once when the cache is cold. The returned values are read-only.
func (c *Cache[G, R]) loadAllNames(loader func() ([]string, error)) ([]string, map[string]struct{}, error) {
	c.ruleMu.RLock()
	if c.allNamesOK {
		names, set := c.allNames, c.allNamesSet
		c.ruleMu.RUnlock()
		return names, set, nil
	}
	c.ruleMu.RUnlock()

	c.ruleMu.Lock()
	defer c.ruleMu.Unlock()
	if c.allNamesOK {
		return c.allNames, c.allNamesSet, nil
	}
	names, err := loader()
	if err != nil {
		return nil, nil, err
	}
	c.allNames = clone(names)
	c.allNamesSet = nameSet(c.allNames)
	c.allNamesOK = true
	return c.allNames, c.allNamesSet, nil
}

// InvalidateUser clears all cached permission data for uid.
func (c *Cache[G, R]) InvalidateUser(uid int32) {
	c.ruleMu.Lock()
	defer c.ruleMu.Unlock()
	c.groupMu.Lock()
	defer c.groupMu.Unlock()
	delete(c.ruleList, uid)
	delete(c.ruleNames, uid)
	delete(c.ruleNameSets, uid)
	delete(c.groupList, uid)
	c.allNames = nil
	c.allNamesSet = nil
	c.allNamesOK = false
}

// InvalidateAll clears all cached permission data.
func (c *Cache[G, R]) InvalidateAll() {
	c.ruleMu.Lock()
	defer c.ruleMu.Unlock()
	c.groupMu.Lock()
	defer c.groupMu.Unlock()
	clear(c.ruleList)
	clear(c.ruleNames)
	clear(c.ruleNameSets)
	clear(c.groupList)
	c.allNames = nil
	c.allNamesSet = nil
	c.allNamesOK = false
}

func clone[T any](values []T) []T {
	return append([]T(nil), values...)
}

func nameSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	return set
}
