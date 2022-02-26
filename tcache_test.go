package tinycache

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

const expire = time.Millisecond * 900

const key = "key0"
const value = "value0"

const key1 = "key1"
const value1 = "value1"

const key2 = "key2"
const value2 = "value2"

func shouldExist(cache *cache, key interface{}, t *testing.T) {
	_, found := cache.Get(key)
	if !found {
		t.Error("Can't getting value that should exist using key:", key)
	}
}

func shouldNotExist(cache *cache, key interface{}, t *testing.T) {
	_, found := cache.Get(key)
	if found {
		t.Error("Getting a value that shouldn't exist using key:", key)
	}
}

func TestSetAndExpireWithGlobalExpiration(t *testing.T) {
	cache := NewCacheBuilder().WithExpiration(expire).Build()
	cache.Put(key, value)
	<-time.After(expire + 5*time.Millisecond)
	shouldNotExist(cache, key, t)
}

func TestSetAndExpireWithSpecExpiration(t *testing.T) {
	cache := NewCacheBuilder().WithExpiration(expire).Build()
	cache.PutWithTtl(key, value, expire+50*time.Millisecond)
	<-time.After(expire + 5*time.Millisecond)
	shouldExist(cache, key, t)
	<-time.After(expire + 50*time.Millisecond)
	shouldNotExist(cache, key, t)
}

func TestNeverExpire(t *testing.T) {
	cache := NewCacheBuilder().WithExpiration(expire).Build()
	cache.PutWithTtl(key, value, NeverExpire)
	<-time.After(expire + 5*time.Millisecond)
	shouldExist(cache, key, t)
}

func TestNeverExpire_2(t *testing.T) {
	cache := NewCacheBuilder().WithExpiration(3 * expire).WithMaxSize(2).Build()
	cache.PutWithTtl(key, value, NeverExpire)
	<-time.After(expire)
	cache.Put(key1, value1)
	<-time.After(expire + 5*time.Millisecond)
	cache.Put(key2, value2)
	shouldExist(cache, key, t)
	shouldNotExist(cache, key1, t)
}

func TestNotifyListenerTriggerByExpired(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	ch := make(chan RemoveReason)
	cache := NewCacheBuilder().WithExpiration(expire).WithRemoveListener(func(key, value interface{}, reason RemoveReason) {
		wg.Done()
		ch <- reason
	}).Build()
	cache.Put(key, value)
	if waitTimeout(&wg, expire*2) {
		t.Error("RemoveListener should be called")
	}
	if reason := <-ch; reason != Expired {
		t.Error("RemoveReason should be `Expired`")
	}
}

func TestNotifyListenerTriggerByMaxSize(t *testing.T) {
	ch := make(chan interface{})
	cache := NewCacheBuilder().WithExpiration(expire).WithMaxSize(1).WithRemoveListener(func(key, value interface{}, reason RemoveReason) {
		ch <- reason
		ch <- key
		ch <- value
	}).Build()
	cache.Put(key, value)
	cache.Put(key1, value1)
	if reason := <-ch; reason != Evict {
		t.Error("RemoveReason should be `Evict`")
	}
	if acceptKey := <-ch; acceptKey != key {
		t.Error("Key should be `key`")
	}
	if acceptValue := <-ch; acceptValue != value {
		t.Error("Value should be `value`")
	}
}

func TestExpirePolicy_CREATED(t *testing.T) {
	ch := make(chan interface{})
	cache := NewCacheBuilder().WithExpiration(expire * 3).WithMaxSize(2).
		WithExpirePolicy(Created).WithRemoveListener(func(key, value interface{}, reason RemoveReason) {
		ch <- key
		ch <- value
	}).Build()
	cache.Put(key, value)
	<-time.After(100)
	cache.Put(key1, value1)
	<-time.After(100)
	cache.Put(key2, value2)
	if acceptKey := <-ch; acceptKey != key {
		t.Error("Key should be `key`")
	}
	if acceptValue := <-ch; acceptValue != value {
		t.Error("Value should be `value`")
	}
}

func TestExpirePolicy_CREATED_GET(t *testing.T) {
	ch := make(chan interface{})
	cache := NewCacheBuilder().WithExpiration(expire * 3).WithMaxSize(2).
		WithExpirePolicy(Created).WithRemoveListener(func(key, value interface{}, reason RemoveReason) {
		ch <- key
		ch <- value
	}).Build()
	cache.Put(key, value)
	<-time.After(expire)
	cache.Get(key)
	cache.Put(key1, value1)
	<-time.After(100)
	cache.Put(key2, value2)
	if acceptKey := <-ch; acceptKey != key {
		t.Error("Key should be `key`")
	}
	if acceptValue := <-ch; acceptValue != value {
		t.Error("Value should be `value`")
	}
}

func TestExpirePolicy_ACCESSED(t *testing.T) {
	ch := make(chan interface{})
	cache := NewCacheBuilder().
		WithExpiration(expire * 3).WithMaxSize(2).
		WithExpirePolicy(Created).WithRemoveListener(func(key, value interface{}, reason RemoveReason) {
		ch <- key
		ch <- value
	}).Build()
	cache.Put(key, value)
	<-time.After(expire)
	cache.Put(key1, value1)
	<-time.After(expire)
	cache.Get(key)
	cache.Put(key2, value2)
	if acceptKey := <-ch; acceptKey != key {
		t.Error("Key should be `key1`")
	}
	if acceptValue := <-ch; acceptValue != value {
		t.Error("Value should be `value1`")
	}
}

func TestConcurrency_2(t *testing.T) {
	triggerCnt := 0
	lock := sync.Mutex{}
	cache := NewCacheBuilder().
		WithExpiration(expire).WithRemoveListener(func(key, value interface{}, reason RemoveReason) {
		lock.Lock()
		defer lock.Unlock()
		triggerCnt++
	}).Build()
	wg := sync.WaitGroup{}
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			cache.Put(rand.Float32(), struct{}{})
			wg.Done()
		}()
	}
	wg.Wait()
	<-time.After(2 * expire)
	if triggerCnt != 1000 {
		t.Error("TriggerCnt should be 1000, but", triggerCnt)
	}
}

func TestConcurrency(t *testing.T) {
	cache := NewCacheBuilder().
		WithExpiration(expire).Build()
	wg := sync.WaitGroup{}
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			cache.Put(rand.Float32(), struct{}{})
			wg.Done()
		}()
	}
	wg.Wait()
	<-time.After(expire * 2)
	if cache.Size() != 0 {
		t.Error("Map should be empty")
	}
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}
