package dedupe

import "sync"

type InMemory struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func NewInMemory() *InMemory {
	return &InMemory{seen: make(map[string]struct{})}
}

func (d *InMemory) Seen(hash string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.seen[hash]
	return ok
}

func (d *InMemory) Mark(hash string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[hash] = struct{}{}
}
