package tinycache

import (
	"github.com/huandu/skiplist"
	"math"
	"sync"
	"time"
)

type CacheBuilder struct {
	cache *cache
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

func (b *CacheBuilder) Build() *cache {
	return b.cache
}
