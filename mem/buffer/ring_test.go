package buffer

import (
	"fmt"
	"testing"
	"unsafe"
)

func TestSize(t *testing.T) {
	if unsafe.Sizeof(innerEntry[entity]{}) != cpuCacheLineSize {
		t.Fatal("innerEntry is over a cpu cache line")
	}
}

type entity struct {
	val int
}

func TestBase(t *testing.T) {
	r := NewRingBuffer[entity](6)

	for i := range 7 {
		r.Put(&entity{
			val: i,
		})
	}

	for range 7 {
		e, err := r.Get()
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("Get: %v\n", e)
	}
}
