package utils

import (
	"sync"
	"time"

	"math/rand"
)

func Shuffle[T any](slice []T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // 局部随机数生成器
	n := len(slice)
	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)                      // 使用局部随机数生成器
		slice[i], slice[j] = slice[j], slice[i] // 交换元素
	}
}

type Heap[T any] struct {
	data []T
	less func(T, T) bool
	mtx  sync.Mutex
}

func NewHeap[T any](less func(T, T) bool) *Heap[T] {
	return &Heap[T]{less: less, data: []T{}}
}

func (hp *Heap[T]) Push(val T) {
	hp.mtx.Lock()
	defer hp.mtx.Unlock()

	hp.data = append(hp.data, val)
	hp.siftUp(len(hp.data) - 1)
}

func (hp *Heap[T]) Pop() {
	hp.mtx.Lock()
	defer hp.mtx.Unlock()

	n := len(hp.data)
	if n == 0 {
		panic("heap is empty")
	}

	hp.swap(0, n-1)
	hp.data = hp.data[:n-1]
	hp.siftDown(0)
}

func (hp *Heap[T]) Peek() T {
	hp.mtx.Lock()
	defer hp.mtx.Unlock()

	if len(hp.data) == 0 {
		panic("heap is empty")
	}
	return hp.data[0]
}

func (hp *Heap[T]) Size() int {
	hp.mtx.Lock()
	defer hp.mtx.Unlock()

	return len(hp.data)
}

func (hp *Heap[T]) Empty() bool {
	hp.mtx.Lock()
	defer hp.mtx.Unlock()

	return len(hp.data) == 0
}

func (hp *Heap[T]) left(i int) int {
	return 2*i + 1
}

func (hp *Heap[T]) right(i int) int {
	return 2*i + 2
}

func (hp *Heap[T]) parent(i int) int {
	return (i - 1) / 2
}

func (hp *Heap[T]) swap(i int, j int) {
	hp.data[i], hp.data[j] = hp.data[j], hp.data[i]
}

func (hp *Heap[T]) siftUp(i int) {
	for {
		p := hp.parent(i)
		if p < 0 || !hp.less(hp.data[i], hp.data[p]) {
			break
		}

		hp.swap(i, p)
		i = p
	}
}

func (hp *Heap[T]) siftDown(i int) {
	for {
		min := i
		left, right := hp.left(i), hp.right(i)
		if left < len(hp.data) && hp.less(hp.data[left], hp.data[min]) {
			min = left
		}
		if right < len(hp.data) && hp.less(hp.data[right], hp.data[min]) {
			min = right
		}

		if min == i {
			break
		}
		hp.swap(min, i)
		i = min
	}
}
