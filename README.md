# tinycache
**This project is used to learn the Go language.**

Concurrency-safe k-v store which supports expiration and a few other features:

1.Set the global expiration time or the expiration time of a single key.

2.ExpirePolicy: 

* Accessed: The expiration time is refreshed each time the key is accessed.

* Created: The expiration time depends only on the time the key was created.

3.Set RemoveListener callback.(Expired or Evict because of maxsize is reached).

4.Set MaxSize, when maxSize is reached, the current fastest to expire key is removed.


**Design:**

An additional skiplist(http://github.com/huandu/skiplist) is used to maintain the data, and a timer is set each time on the element that will expire the soonest.

**Demo**
```
// go get github.com/zysaaa/tinycache
// import "github.com/zysaaa/tinycache"

cache := tinycache.NewCacheBuilder().
		WithExpiration(expire * 3).
		WithMaxSize(2).
		WithExpirePolicy(Created).
		WithRemoveListener(func(key, value interface{}, reason RemoveReason) {
	}).Build()
cache.Put("a", "a")
cache.PutWithTtl("b", "b", 500*time.Millisecond)
```