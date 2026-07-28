// Package permissioncache contains the private, shared cache mechanics used
// by the admin and frontend permission models.
package permissioncache

import "sync"

// Cache stores group membership and the derived permission rules for each
// user. Rule reloads hold the rule lock while loading groups, so callers must
// keep the rule-then-group lock order when adding cache operations.
type Cache[G any, R any] struct {
	groupMu   sync.RWMutex
	groupList map[int32][]G
	ruleMu    sync.RWMutex
	ruleList  map[int32][]R
	ruleNames map[int32][]string
}

func New[G any, R any]() *Cache[G, R] {
	return &Cache[G, R]{
		groupList: make(map[int32][]G),
		ruleList:  make(map[int32][]R),
		ruleNames: make(map[int32][]string),
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
	c.ruleList[uid] = clone(rules)
	c.ruleNames[uid] = clone(names)
	return clone(names), nil
}

// Rules returns a copy of the cached rules for uid.
func (c *Cache[G, R]) Rules(uid int32) ([]R, bool) {
	c.ruleMu.RLock()
	defer c.ruleMu.RUnlock()
	rules, ok := c.ruleList[uid]
	return clone(rules), ok
}

// RuleNames returns a copy of the cached rule names for uid.
func (c *Cache[G, R]) RuleNames(uid int32) []string {
	c.ruleMu.RLock()
	defer c.ruleMu.RUnlock()
	return clone(c.ruleNames[uid])
}

// InvalidateUser clears all cached permission data for uid.
func (c *Cache[G, R]) InvalidateUser(uid int32) {
	c.ruleMu.Lock()
	defer c.ruleMu.Unlock()
	c.groupMu.Lock()
	defer c.groupMu.Unlock()
	delete(c.ruleList, uid)
	delete(c.ruleNames, uid)
	delete(c.groupList, uid)
}

// InvalidateAll clears all cached permission data.
func (c *Cache[G, R]) InvalidateAll() {
	c.ruleMu.Lock()
	defer c.ruleMu.Unlock()
	c.groupMu.Lock()
	defer c.groupMu.Unlock()
	clear(c.ruleList)
	clear(c.ruleNames)
	clear(c.groupList)
}

func clone[T any](values []T) []T {
	return append([]T(nil), values...)
}
