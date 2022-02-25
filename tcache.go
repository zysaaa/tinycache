package tinycache

import (
	"fmt"
	"github.com/huandu/skiplist"
	"math"
	"sync"
	"time"
)

// todo test

type cache struct {
	lock             sync.RWMutex
	data             map[interface{}]*Item
	skipList         *skiplist.SkipList
	globalExpiration time.Duration  // Configurable, -1 means never expire(which is default value)
	removeListener   RemoveListener // Configurable, nil-able
	maxSize          int            // Configurable, default value: math.MaxInt
	expirePolicy     ExpirePolicy   // Configurable, default value: ACCESSED
}

type CacheBuilder struct {
	cache *cache
}

func (b *CacheBuilder) WithExpiration(globalExpiration time.Duration) *CacheBuilder {
	b.cache.globalExpiration = globalExpiration
	return b
}

func (b *CacheBuilder) WithRemoveListener(removeListener RemoveListener) *CacheBuilder {
	b.cache.removeListener = removeListener
	return b
}

func (b *CacheBuilder) WithMaxSize(maxSize int) *CacheBuilder {
	b.cache.maxSize = maxSize
	return b
}

func (b *CacheBuilder) WithExpirePolicy(expirePolicy ExpirePolicy) *CacheBuilder {
	b.cache.expirePolicy = expirePolicy
	return b
}

func NewCacheBuilder() *CacheBuilder {
	return &CacheBuilder{
		cache: &cache{
			data:             map[interface{}]*Item{},
			lock:             sync.RWMutex{},
			skipList:         skiplist.New(comparableFunc),
			maxSize:          math.MaxInt,
			expirePolicy:     Accessed,
			globalExpiration: -1,
		}}
}

func (b *CacheBuilder) Build() *cache {
	return b.cache
}

type RemoveReason int
type RemoveListener func(key, value interface{}, reason RemoveReason)

func (cache *cache) SetRemoveListener(f func(key, value interface{}, reason RemoveReason)) {
	// Do we need lock here?

	//cache.lock.Lock()
	//defer cache.lock.Unlock()
	cache.removeListener = f
}

const (
	Expired RemoveReason = iota
	Evict
)

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
	return cache.put(key, value, cache.globalExpiration)
}

func (cache *cache) Size() int {
	cache.lock.RLock()
	defer cache.lock.RUnlock()

	fmt.Println("size is : ", len(cache.data), cache.skipList.Len())
	return len(cache.data)
}

func (cache *cache) PutWithTtl(key interface{}, value interface{}, ttl time.Duration) interface{} {
	return cache.put(key, value, ttl)
}

func (cache *cache) put(key interface{}, value interface{}, ttl time.Duration) interface{} {
	cache.lock.Lock()
	defer cache.lock.Unlock()

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
		// Remove "smallest" item from skipList
		front := cache.skipList.Front()
		cache.deleteAndNotifyAndReschedule(front.Key().(*Item), Evict)
	}

	cache.data[key] = &item
	if ttl != NeverExpire {
		cache.skipList.Set(&item, struct{}{})
		//fmt.Println(item.deadline.Sub(now))
		//cache.skipList.Set(&item, struct{}{})
		if cache.skipList.Len() == 1 {
			// 启动定时任务
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
	// delete
	cache.delete(item)
	// notify
	cache.notifyListener(item, reason)
	// re-schedule
	cache.reschedule()
}

func (cache *cache) notifyListener(item *Item, reason RemoveReason) {
	// Start a new go-routine
	if cache.removeListener != nil {
		go func() {
			cache.removeListener(item.key, item.value, reason)
		}()
	}
}

func (cache *cache) delete(item *Item) {
	delete(cache.data, item.key)
	cache.skipList.Remove(item)
}

func (cache *cache) reschedule() {
	now := time.Now()
	for {
		if cache.skipList.Len() == 0 {
			break
		}
		front := cache.skipList.Front()
		if front.Key().(*Item).deadline.Before(now) || front.Key().(*Item).deadline.Equal(now) {
			cache.delete(front.Key().(*Item))
			cache.notifyListener(front.Key().(*Item), Expired)
		} else {
			front.Key().(*Item).cleaner = time.AfterFunc(front.Key().(*Item).deadline.Sub(now), func() {
				cache.lock.Lock()
				item := front.Key().(*Item)
				defer cache.lock.Unlock()
				// delete
				cache.delete(item)
				// notify
				cache.notifyListener(item, Expired)
				// re-schedule
				cache.reschedule()
			})
			break
		}
	}
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
