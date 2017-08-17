package stringcmap

import (
	"fmt"
	"sync"

	"github.com/OneOfOne/cmap"
)

// DefaultShardCount is the default number of shards to use when New() or NewFromJSON() are called.
// The default is 256.
const DefaultShardCount = cmap.DefaultShardCount

// CMap is a concurrent safe sharded map to scale on multiple cores.
type CMap struct {
	shards []lmap
	mod    uint32
}

// New is an alias for NewSize(DefaultShardCount)
func New() *CMap { return NewSize(DefaultShardCount) }

// NewSize returns a CMap with the specific shardSize, note that for performance reasons,
// shardCount must be a power of 2.
// Higher shardCount will improve concurrency but will consume more memory.
func NewSize(shardCount int) *CMap {
	// must be a power of 2
	if shardCount < 1 {
		shardCount = DefaultShardCount
	} else if shardCount&(shardCount-1) != 0 {
		panic("shardCount must be a power of 2")
	}

	cm := &CMap{
		shards: make([]lmap, shardCount),
		mod:    uint32(shardCount) - 1,
	}

	for i := range cm.shards {
		cm.shards[i].m = make(map[string]interface{})
	}

	return cm
}

func (cm *CMap) shard(key string) *lmap {
	h := cmap.Fnv32(key)
	return &cm.shards[h&cm.mod]
}

// Get is the equivalent of `val := map[key]`.
func (cm *CMap) Get(key string) (val interface{}) {
	return cm.shard(key).Get(key)
}

// GetOK is the equivalent of `val, ok := map[key]`.
func (cm *CMap) GetOK(key string) (val interface{}, ok bool) {
	return cm.shard(key).GetOK(key)
}

// Set is the equivalent of `map[key] = val`.
func (cm *CMap) Set(key string, val interface{}) {
	cm.shard(key).Set(key, val)
}

// SetIfNotExists will only assign val to key if it wasn't already set.
// Use `CMap.Update` if you need more logic.
func (cm *CMap) SetIfNotExists(key string, val interface{}) (set bool) {
	cm.Update(key, func(oldVal interface{}) (newVal interface{}) {
		switch oldVal.(type) {
		case nil:
			return newVal
		default:
			return oldVal
		}
	})
	return
}

// Has is the equivalent of `_, ok := map[key]`.
func (cm *CMap) Has(key string) bool { return cm.shard(key).Has(key) }

// Delete is the equivalent of `delete(map, key)`.
func (cm *CMap) Delete(key string) { cm.shard(key).Delete(key) }

// DeleteAndGet is the equivalent of `oldVal := map[key]; delete(map, key)`.
func (cm *CMap) DeleteAndGet(key string) interface{} { return cm.shard(key).DeleteAndGet(key) }

// Update calls `fn` with the key's old value (or nil if it didn't exist) and assign the returned value to the key.
// The shard containing the key will be locked, it is NOT safe to call other cmap funcs inside `fn`.
func (cm *CMap) Update(key string, fn func(oldval interface{}) (newval interface{})) {
	cm.shard(key).Update(key, fn)
}

// Swap is the equivalent of `oldVal, map[key] = map[key], newVal`.
func (cm *CMap) Swap(key string, val interface{}) interface{} {
	return cm.shard(key).Swap(key, val)
}

// Keys returns a slice of all the keys of the map.
func (cm *CMap) Keys() []string {
	out := make([]string, 0, cm.Len())
	for i := range cm.shards {
		sh := &cm.shards[i]
		sh.l.RLock()
		for k := range sh.m {
			out = append(out, k)
		}
		sh.l.RUnlock()
	}
	return out
}

// ForEach loops over all the key/values in all the shards in order.
// You can break early by returning an error.
// it is safe to change the map inside fn.
func (cm *CMap) ForEach(fn func(key string, val interface{}) error) error {
	for i := range cm.shards {
		if err := cm.shards[i].ForEach(fn); err != nil {
			return err
		}
	}
	return nil
}

// KV is returned from the Iter channel.
type KV struct {
	Key   string
	Value interface{}
}

// Iter returns a channel to be used in for range.
// **Warning** that breaking early will leak up to cm.NumShards() goroutines, use IterWithCancel if you intend to break early.
// It is safe to modify the map while using the iterator.
func (cm *CMap) Iter(buffer int) <-chan *KV {
	ch, _ := cm.IterWithCancel(buffer)
	return ch
}

var errBreak = fmt.Errorf("break")

// IterWithCancel returns a channel to be used in for range and
// a cancelFn that can be called at any time to cleanly exit early.
// Note that cancelFn will block until all the writers are notified.
// It is safe to modify the map while using the iterator.
func (cm *CMap) IterWithCancel(buffer int) (kvChan <-chan *KV, cancelFn func()) {
	var (
		wg       sync.WaitGroup
		ch       = make(chan *KV, buffer)
		cancelCh = make(chan struct{})
	)

	kvChan, cancelFn = ch, func() {
		select {
		case <-cancelCh:
		default:
			close(cancelCh)
			for range ch {
			}
		}
	}

	wg.Add(len(cm.shards))

	go func() {
		for i := range cm.shards {
			go func(i int) {
				cm.shards[i].ForEach(func(k string, v interface{}) error {
					select {
					case <-cancelCh:
						return errBreak
					case ch <- &KV{k, v}:
						return nil
					}
				})
				wg.Done()
			}(i)
		}
		wg.Wait()
		close(ch)
		cancelFn()
	}()

	return
}

// Len returns the number of elements in the map.
func (cm *CMap) Len() int {
	ln := 0
	for i := range cm.shards {
		ln += cm.shards[i].Len()
	}
	return ln
}

// NumShards returns the number of shards in the map.
func (cm *CMap) NumShards() int { return len(cm.shards) }
