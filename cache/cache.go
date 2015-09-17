package cache

import (
	"fmt"
	"sync"

	"github.com/outdoorsy/gorp"
	"github.com/outdoorsy/mc"
)

const defaultCacheExpirationTime = 604800 // one week

type tableKeys struct {
	keys    map[*gorp.TableMap][]string
	keyLock sync.RWMutex
}

func (t tableKeys) get(table *gorp.TableMap) []string {
	t.keyLock.Lock()
	if t.keys == nil {
		t.keys = map[*gorp.TableMap][]string{}
	}
	keys := t.keys[table]
	t.keyLock.Unlock()
	return keys
}

func (t tableKeys) add(table *gorp.TableMap, key string) {
	t.keyLock.Lock()
	if t.keys == nil {
		t.keys = map[*gorp.TableMap][]string{}
	}
	t.keys[table] = append(t.keys[table], key)
	t.keyLock.Unlock()
}

func (t tableKeys) drop(table *gorp.TableMap) {
	t.keyLock.Lock()
	if t.keys == nil {
		t.keys = map[*gorp.TableMap][]string{}
	}
	delete(t.keys, table)
	t.keyLock.Unlock()
}

// Memcachier is a gorq.Cache using github.com/memcachier/mc.
type Memcachier struct {
	Conn          mc.Cacher
	cacheables    map[*gorp.TableMap]bool
	keys          tableKeys
	relationships map[*gorp.TableMap][]*gorp.TableMap
}

func (m *Memcachier) SetCacheable(table *gorp.TableMap, cacheable bool) {
	if m.cacheables == nil {
		m.cacheables = map[*gorp.TableMap]bool{}
	}
	m.cacheables[table] = cacheable
}

func (m *Memcachier) Cacheable(table *gorp.TableMap) bool {
	if m.cacheables == nil {
		return false
	}
	return m.cacheables[table]
}

// Relate relates source to target, so that whenever keys for
// source are dropped, keys for target are dropped as well.
func (m *Memcachier) Relate(source, target *gorp.TableMap) {
	if m.relationships == nil {
		m.relationships = map[*gorp.TableMap][]*gorp.TableMap{}
	}
	m.relationships[source] = append(m.relationships[source], target)
}

// related returns all tables related to table.
func (m *Memcachier) related(table *gorp.TableMap) []*gorp.TableMap {
	if m.relationships == nil {
		return nil
	}
	return m.relationships[table]
}

// Set sets a key:value pair in the cache, and relates that key to
// all entries in the tables slice.
func (m *Memcachier) Set(tables []*gorp.TableMap, key, value string) error {
	fmt.Println("setting key", key, len(value))
	_, err := m.Conn.Set(key, value, 0, defaultCacheExpirationTime, 0)
	fmt.Println("setting key err?", err)
	for _, t := range tables {
		m.keys.add(t, key)
	}
	return err
}

// Get returns the value for key.
func (m *Memcachier) Get(key string) (string, error) {
	fmt.Println("getting key", key)
	s, _, _, err := m.Conn.Get(key)
	if err != nil {
		fmt.Println("error getting key", err)
		return "", err
	}
	fmt.Println("got key", key)
	return s, err
}

// DropEntries should drop all entries in the cache related to
// tables.
func (m *Memcachier) DropEntries(tables []*gorp.TableMap) error {
	droppedEntries := map[*gorp.TableMap]bool{}
	for _, t := range tables {
		if err := m.dropTableEntries(t, droppedEntries); err != nil {
			return err
		}
	}
	return nil
}

func (m *Memcachier) dropTableEntries(table *gorp.TableMap, dropped map[*gorp.TableMap]bool) error {
	if done := dropped[table]; done {
		return nil
	}
	dropped[table] = true
	for _, key := range m.keys.get(table) {
		err := m.Conn.Del(key)
		if err != nil {
			return err
		}
	}
	// It would be easier to use DropEntries here, but we need to use
	// the same value for dropped (to handle cyclic relationships in
	// m.relationships), and we don't really need yet another helper
	// method.
	for _, t := range m.related(table) {
		if err := m.dropTableEntries(t, dropped); err != nil {
			return err
		}
	}
	m.keys.drop(table)
	return nil
}
