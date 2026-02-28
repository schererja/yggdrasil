package auth

import "sync"

type Registry struct {
	mu          sync.RWMutex
	permissions map[string]Permission
}

func NewRegistry() *Registry {
	return &Registry{permissions: make(map[string]Permission)}
}

func (r *Registry) Register(p Permission) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.permissions[p.Key] = p
}

func (r *Registry) Get(key string) (Permission, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.permissions[key]
	return p, ok
}

func (r *Registry) List() []Permission {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Permission, 0, len(r.permissions))
	for _, p := range r.permissions {
		out = append(out, p)
	}
	return out
}

var GlobalRegistry = NewRegistry()

func RegisterPermission(p Permission) {
	GlobalRegistry.Register(p)
}
