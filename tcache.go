package tinycache

import (
	"github.com/huandu/skiplist"
	"sync"
	"time"
)

type cache struct {
	lock             sync.RWMutex
	data             map[interface{}]*Item
	skipList         *skiplist.SkipList
	globalExpiration time.Duration  // Configurable, -1 means never expire(which is default value)
	removeListener   RemoveListener // Configurable, nil-able
	maxSize          int            // Configurable, default value: math.MaxInt
	expirePolicy     ExpirePolicy   // Configurable, default value: ACCESSED
}

type RemoveReason int

const (
	Expired RemoveReason = iota
	Evict
)

type RemoveListener func(key, value interface{}, reason RemoveReason)

type ExpirePolicy int

const (
	Accessed ExpirePolicy = iota
	Created
)

const NeverExpire = -1

type Item struct {
	key      interface{}
	value    interface{}
	deadline time.Time
	ttl      time.Duration
	cleaner  *time.Timer
}

func (cache *cache) Put(key interface{}, value interface{}) interface{} {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	return cache.put(key, value, cache.globalExpiration)
}

func (cache *cache) PutWithTtl(key interface{}, value interface{}, ttl time.Duration) interface{} {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	return cache.put(key, value, ttl)
}

func (cache *cache) Get(key interface{}) (interface{}, bool) {
	cache.lock.RLock()
	defer cache.lock.RUnlock()
	data, ok := cache.data[key]
	if ok {
		if cache.expirePolicy == Accessed {
			cache.skipList.Remove(data)
			data.deadline = time.Now().Add(data.ttl)
			cache.skipList.Set(data, struct{}{})
			cache.data[data.key] = data
		}
		return data.value, true
	}
	return nil, false
}

func (cache *cache) Size() int {
	cache.lock.RLock()
	defer cache.lock.RUnlock()
	return len(cache.data)
}

func (cache *cache) put(key interface{}, value interface{}, ttl time.Duration) interface{} {
	now := time.Now()
	original, ok := cache.data[key]
	if ok {
		if original.cleaner != nil {
			original.cleaner.Stop()
		}
	}
	item := Item{
		key:      key,
		value:    value,
		deadline: now.Add(ttl),
		ttl:      ttl,
	}
	if len(cache.data) == cache.maxSize {
		front := cache.skipList.Front()
		cache.deleteAndNotifyAndReschedule(front.Key().(*Item), Evict)
	}
	cache.data[key] = &item
	if ttl != NeverExpire {
		cache.skipList.Set(&item, struct{}{})
		if cache.skipList.Len() == 1 {
			item.cleaner = time.AfterFunc(item.deadline.Sub(now), func() {
				cache.lock.Lock()
				defer cache.lock.Unlock()
				cache.deleteAndNotifyAndReschedule(&item, Expired)
			})
		} else {
			first := cache.skipList.Front()
			if first.Key().(*Item).deadline.After(item.deadline) {
				first.Key().(*Item).cleaner.Stop()
				item.cleaner = time.AfterFunc(item.deadline.Sub(now), func() {
					cache.lock.Lock()
					defer cache.lock.Unlock()
					cache.deleteAndNotifyAndReschedule(&item, Expired)
				})
			}
		}
	}
	return original
}

func (cache *cache) deleteAndNotifyAndReschedule(item *Item, reason RemoveReason) {
	if cache.delete(item) {
		cache.notifyListener(item, reason)
	}
	cache.reschedule()
}

func (cache *cache) notifyListener(item *Item, reason RemoveReason) {
	if cache.removeListener != nil {
		go func() {
			cache.removeListener(item.key, item.value, reason)
		}()
	}
}

func (cache *cache) delete(item *Item) bool {
	delete(cache.data, item.key)
	return cache.skipList.Remove(item) != nil
}

func (cache *cache) reschedule() {
	now := time.Now()
	for cache.skipList.Len() != 0 {
		front := cache.skipList.Front()
		if front.Key().(*Item).deadline.After(now) {
			front.Key().(*Item).cleaner = time.AfterFunc(front.Key().(*Item).deadline.Sub(now), func() {
				cache.lock.Lock()
				item := front.Key().(*Item)
				defer cache.lock.Unlock()
				cache.deleteAndNotifyAndReschedule(item, Expired)
			})
			break
		} else {
			if cache.delete(front.Key().(*Item)) {
				cache.notifyListener(front.Key().(*Item), Expired)
			}
		}
	}
}

var comparableFunc = skiplist.GreaterThanFunc(func(k1, k2 interface{}) int {
	if k1.(*Item).key == k2.(*Item).key {
		return 0
	}
	if k1.(*Item).deadline.After(k2.(*Item).deadline) {
		return 1
	} else {
		return -1
	}
})
