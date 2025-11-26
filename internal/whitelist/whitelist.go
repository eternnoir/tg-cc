package whitelist

import "sync"

// Whitelist manages allowed user IDs for a bot
type Whitelist struct {
	mu      sync.RWMutex
	allowed map[int64]struct{}
}

// New creates a new Whitelist with the given user IDs
func New(userIDs []int64) *Whitelist {
	w := &Whitelist{
		allowed: make(map[int64]struct{}),
	}
	for _, id := range userIDs {
		w.allowed[id] = struct{}{}
	}
	return w
}

// IsAllowed checks if a user ID is in the whitelist
// If the whitelist is empty, all users are allowed
func (w *Whitelist) IsAllowed(userID int64) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// If whitelist is empty, allow all users
	if len(w.allowed) == 0 {
		return true
	}

	_, ok := w.allowed[userID]
	return ok
}

// Add adds a user ID to the whitelist
func (w *Whitelist) Add(userID int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.allowed[userID] = struct{}{}
}

// Remove removes a user ID from the whitelist
func (w *Whitelist) Remove(userID int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.allowed, userID)
}

// List returns all user IDs in the whitelist
func (w *Whitelist) List() []int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	ids := make([]int64, 0, len(w.allowed))
	for id := range w.allowed {
		ids = append(ids, id)
	}
	return ids
}
