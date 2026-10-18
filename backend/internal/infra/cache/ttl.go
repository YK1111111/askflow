package cache

import (
	"hash/fnv"
	"time"
)

func ttlWithJitter(key string, base, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return base
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return base + time.Duration(hash.Sum32()%uint32(jitter/time.Millisecond+1))*time.Millisecond
}
