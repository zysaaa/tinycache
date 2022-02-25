# tinycache
Concurrency-safe k-v store which supports expiration and a few other features:

1.Set the global expiration time or the expiration time of a single key.

2.ExpirePolicy: 

* Accessed: The expiration time is refreshed each time the KEY is accessed.

* Created: The expiration time depends only on the time the key was created.

3.Set RemoveListener callback.(Expired or Evict because of maxsize is reached).

4.Set MaxSize, when maxSize is reached, the current fastest to expire key is removed.


**Design:**

An additional skiplist is used to maintain the data, and a timer is set each time on the element that will expire the soonest.

**Demo**
```


```