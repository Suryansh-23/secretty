package cache

import (
	"container/list"
	"sync"
	"time"

	"github.com/suryansh-23/secretty/internal/types"
)

// SecretRecord stores a redacted secret for copy-without-render.
type SecretRecord struct {
	ID        int
	Type      types.SecretType
	Original  []byte
	RuleName  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Cache stores secrets in-memory with TTL and LRU eviction.
type Cache struct {
	mu         sync.Mutex
	lru        *list.List
	byID       map[int]*list.Element
	maxEntries int
	ttl        time.Duration
	now        func() time.Time
	lastID     int
}

// New creates a new cache with bounds.
func New(maxEntries int, ttl time.Duration) *Cache {
	if maxEntries <= 0 {
		maxEntries = 64
	}
	return &Cache{
		lru:        list.New(),
		byID:       make(map[int]*list.Element),
		maxEntries: maxEntries,
		ttl:        ttl,
		now:        time.Now,
	}
}

// NextID returns a new event ID.
func (c *Cache) NextID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastID++
	return c.lastID
}

// Put stores a record (with ID) if caching is enabled.
func (c *Cache) Put(record SecretRecord) {
	if c == nil {
		return
	}
	if c.ttl <= 0 {
		return
	}
	if len(record.Original) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastID = max(c.lastID, record.ID)
	now := c.now()
	record.CreatedAt = now
	record.ExpiresAt = now.Add(c.ttl)
	if elem, ok := c.byID[record.ID]; ok {
		c.lru.Remove(elem)
	}
	elem := c.lru.PushFront(record)
	c.byID[record.ID] = elem
	c.evictLocked()
}

// GetLast returns the most recent non-expired record.
func (c *Cache) GetLast() (SecretRecord, bool) {
	if c == nil {
		return SecretRecord{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictExpiredLocked()
	for elem := c.lru.Front(); elem != nil; elem = elem.Next() {
		rec := elem.Value.(SecretRecord)
		if !rec.ExpiresAt.IsZero() && c.now().After(rec.ExpiresAt) {
			c.lru.Remove(elem)
			delete(c.byID, rec.ID)
			continue
		}
		c.lru.MoveToFront(elem)
		return rec, true
	}
	return SecretRecord{}, false
}

// Get returns a record by ID.
func (c *Cache) Get(id int) (SecretRecord, bool) {
	if c == nil {
		return SecretRecord{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.byID[id]
	if !ok {
		return SecretRecord{}, false
	}
	rec := elem.Value.(SecretRecord)
	if !rec.ExpiresAt.IsZero() && c.now().After(rec.ExpiresAt) {
		c.lru.Remove(elem)
		delete(c.byID, rec.ID)
		return SecretRecord{}, false
	}
	c.lru.MoveToFront(elem)
	return rec, true
}

// SetTTL updates the TTL for future entries.
func (c *Cache) SetTTL(ttl time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
}

// SetMaxEntries updates the max entries and evicts if needed.
func (c *Cache) SetMaxEntries(maxEntries int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if maxEntries <= 0 {
		maxEntries = 64
	}
	c.maxEntries = maxEntries
	c.evictLocked()
}

func (c *Cache) evictLocked() {
	c.evictExpiredLocked()
	for c.lru.Len() > c.maxEntries {
		back := c.lru.Back()
		if back == nil {
			return
		}
		rec := back.Value.(SecretRecord)
		delete(c.byID, rec.ID)
		c.lru.Remove(back)
	}
}

func (c *Cache) evictExpiredLocked() {
	now := c.now()
	for elem := c.lru.Back(); elem != nil; {
		prev := elem.Prev()
		rec := elem.Value.(SecretRecord)
		if !rec.ExpiresAt.IsZero() && now.After(rec.ExpiresAt) {
			delete(c.byID, rec.ID)
			c.lru.Remove(elem)
		}
		elem = prev
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
