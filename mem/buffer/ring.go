package buffer

import (
	"errors"
	"runtime"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/cpu"
)

var (
	ErrBuffIsEmptry = errors.New("the buffer is emptry")
	ErrBuffIsFull   = errors.New("the buffer is full")
)

const cpuCacheLineSize = unsafe.Sizeof(cpu.CacheLinePad{})

type innerEntry[T any] struct {
	rw  int32 // 0: writeable, 1: readable, 2: writing; 3: reading
	val *T
	_   [cpuCacheLineSize - 16]byte
}

type Ring[T any] struct {
	cap    uint32
	capMod uint32
	_      [cpuCacheLineSize - 8]byte
	head   uint32
	_      [cpuCacheLineSize - 4]byte
	tail   uint32
	_      [cpuCacheLineSize - 4]byte
	data   []innerEntry[T]
}

func (r *Ring[T]) Get() (d *T, err error) {
	var head, tail, nt uint32
	var entry *innerEntry[T]

	for {
		head = atomic.LoadUint32(&r.head)
		tail = atomic.LoadUint32(&r.tail)

		if head == tail {
			return nil, ErrBuffIsEmptry
		}
		nt = (tail + 1) & r.capMod
		atomic.CompareAndSwapUint32(&r.tail, tail, nt)

		entry = &r.data[tail]
	retry:
		if !atomic.CompareAndSwapInt32(&entry.rw, 1, 3) {
			if atomic.LoadInt32(&entry.rw) == 1 {
				goto retry
			}
			runtime.Gosched()
			continue
		}

		d = entry.val

		atomic.CompareAndSwapInt32(&entry.rw, 3, 0)
		return
	}
}

func (r *Ring[T]) Put(d *T) error {
	var head, tail, nh uint32
	var entry *innerEntry[T]

	for {
		head = atomic.LoadUint32(&r.head)
		tail = atomic.LoadUint32(&r.tail)

		nh = (head + 1) & r.capMod
		if nh == tail {
			return ErrBuffIsFull
		}

		atomic.CompareAndSwapUint32(&r.head, head, nh)
		entry = &r.data[head]
	retry:
		if !atomic.CompareAndSwapInt32(&entry.rw, 0, 2) {
			if atomic.LoadInt32(&entry.rw) == 0 {
				goto retry
			}
			runtime.Gosched()
			continue
		}

		entry.val = d
		atomic.CompareAndSwapInt32(&entry.rw, 2, 1)
		return nil
	}
}

func NewRingBuffer[T any](cap uint32) *Ring[T] {
	cap = roundUpToPower2(cap)

	return &Ring[T]{
		cap:    cap,
		capMod: cap - 1,
		data:   make([]innerEntry[T], cap),
	}
}

func roundUpToPower2(x uint32) uint32 {
	x--
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16
	x++
	return x
}
