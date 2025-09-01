package dedupe

import "sync"

// InMemory is a simple, non-persistent dedupe set.
type InMemory struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// NewInMemory constructs an in-memory dedupe set.
func NewInMemory() *InMemory { return &InMemory{seen: make(map[string]struct{})} }

// Seen reports whether hash exists.
func (d *InMemory) Seen(hash string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.seen[hash]
	return ok
}

// Mark records a hash as seen.
func (d *InMemory) Mark(hash string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[hash] = struct{}{}
}
