// AUTO-GENERATED by mapSpec.sh
// DO NOT EDIT

package stringcmap

import "sync"

type lmap struct {
	m map[string]interface{}
	l *sync.RWMutex
}

func newLmap(cap int) *lmap {
	return &lmap{
		m: make(map[string]interface{}, cap),
		l: new(sync.RWMutex),
	}
}

func (lm lmap) Set(key string, v interface{}) {
	lm.l.Lock()
	lm.m[key] = v
	lm.l.Unlock()
}

func (lm lmap) Update(key string, fn func(oldVal interface{}) (newVal interface{})) {
	lm.l.Lock()
	lm.m[key] = fn(lm.m[key])
	lm.l.Unlock()
}

func (lm lmap) Swap(key string, newV interface{}) (oldV interface{}) {
	lm.l.Lock()
	oldV = lm.m[key]
	lm.m[key] = newV
	lm.l.Unlock()
	return
}

func (lm lmap) Get(key string) (v interface{}) {
	lm.l.RLock()
	v = lm.m[key]
	lm.l.RUnlock()
	return
}
func (lm lmap) GetOK(key string) (v interface{}, ok bool) {
	lm.l.RLock()
	v, ok = lm.m[key]
	lm.l.RUnlock()
	return
}

func (lm lmap) Has(key string) (ok bool) {
	lm.l.RLock()
	_, ok = lm.m[key]
	lm.l.RUnlock()
	return
}

func (lm lmap) Delete(key string) {
	lm.l.Lock()
	delete(lm.m, key)
	lm.l.Unlock()
}

func (lm lmap) DeleteAndGet(key string) (v interface{}) {
	lm.l.Lock()
	v = lm.m[key]
	delete(lm.m, key)
	lm.l.Unlock()
	return v
}

func (lm lmap) Len() (ln int) {
	lm.l.RLock()
	ln = len(lm.m)
	lm.l.RUnlock()
	return
}

func (lm lmap) ForEach(keys []string, fn func(key string, val interface{}) error) (err error) {
	lm.l.RLock()
	for key := range lm.m {
		keys = append(keys, key)
	}
	lm.l.RUnlock()

	for _, key := range keys {
		lm.l.RLock()
		val, ok := lm.m[key]
		lm.l.RUnlock()
		if !ok {
			continue
		}
		if err = fn(key, val); err != nil {
			return
		}
	}

	return
}

func (lm lmap) ForEachLocked(fn func(key string, val interface{}) error) (err error) {
	lm.l.RLock()
	defer lm.l.RUnlock()

	for key, val := range lm.m {
		if err = fn(key, val); err != nil {
			return
		}
	}

	return
}
