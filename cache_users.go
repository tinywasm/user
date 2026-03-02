package user

import "sync"

const maxCacheUsers = 1000

type userCache struct {
	mu    sync.RWMutex
	users map[string]*User
	keys  []string // Used for simple FIFO eviction
}

func newUserCache() *userCache {
	return &userCache{
		users: make(map[string]*User),
		keys:  make([]string, 0, maxCacheUsers),
	}
}

func (c *userCache) Get(id string) (*User, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	u, ok := c.users[id]
	return u, ok
}

func (c *userCache) Set(id string, u *User) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.users[id]; !exists {
		if len(c.keys) >= maxCacheUsers {
			// Evict oldest (FIFO)
			oldest := c.keys[0]
			c.keys = c.keys[1:]
			delete(c.users, oldest)
		}
		c.keys = append(c.keys, id)
	}
	c.users[id] = u
}

func (c *userCache) Delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.users[id]; exists {
		delete(c.users, id)
		for i, k := range c.keys {
			if k == id {
				c.keys = append(c.keys[:i], c.keys[i+1:]...)
				break
			}
		}
	}
}

func (c *userCache) InvalidateByRole(roleID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict any user that has this role
	var toDelete []string
	for id, u := range c.users {
		for _, r := range u.Roles {
			if r.ID == roleID {
				toDelete = append(toDelete, id)
				break
			}
		}
	}

	for _, id := range toDelete {
		delete(c.users, id)
		for i, k := range c.keys {
			if k == id {
				c.keys = append(c.keys[:i], c.keys[i+1:]...)
				break
			}
		}
	}
}

func (c *userCache) InvalidateByPermission(permID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict any user that has this permission
	var toDelete []string
	for id, u := range c.users {
		for _, p := range u.Permissions {
			if p.ID == permID {
				toDelete = append(toDelete, id)
				break
			}
		}
	}

	for _, id := range toDelete {
		delete(c.users, id)
		for i, k := range c.keys {
			if k == id {
				c.keys = append(c.keys[:i], c.keys[i+1:]...)
				break
			}
		}
	}
}

func (c *userCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users = make(map[string]*User)
	c.keys = make([]string, 0, maxCacheUsers)
}
