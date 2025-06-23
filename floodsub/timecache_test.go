package floodsub

import (
	"fmt"
	"testing"
	"time"
)

func TestTimeCacheFound(t *testing.T) {
	tc := NewTimeCache(time.Minute)
	defer tc.Close()

	tc.Add("test")

	if !tc.Has("test") {
		t.Fatal("should have this key")
	}
}

func TestTimeCacheExpire(t *testing.T) {
	sweepInterval = 50 * time.Millisecond

	tc := NewTimeCache(200 * time.Millisecond)
	defer tc.Close()
	for i := 0; i < 4; i++ {
		tc.Add(fmt.Sprint(i))
		time.Sleep(time.Millisecond * 50)
	}

	time.Sleep(210 * time.Millisecond)
	for i := 0; i < 4; i++ {
		if tc.Has(fmt.Sprint(i)) {
			t.Fatalf("should have dropped this key %d", i)
		}
	}
}

func TestTimeCacheReaddBeforeExpire(t *testing.T) {
	sweepInterval = 50 * time.Millisecond

	tc := NewTimeCache(200 * time.Millisecond)
	defer tc.Close()
	tc.Add("test")

	time.Sleep(150 * time.Millisecond)
	tc.Add("test")

	time.Sleep(100 * time.Millisecond)

	if tc.Has("test") {
		t.Fatal("should have dropped this from the cache")
	}
	tc.Close()
}

func TestTimeCacheReaddAfterExpire(t *testing.T) {
	sweepInterval = 50 * time.Millisecond

	tc := NewTimeCache(200 * time.Millisecond)
	defer tc.Close()
	tc.Add("test")

	time.Sleep(300 * time.Millisecond)
	tc.Add("test")

	time.Sleep(50 * time.Millisecond)

	if !tc.Has("test") {
		t.Fatal("should have this key")
	}
}
