package agilepool

import "sync/atomic"

type LinkedList[T any] struct {
	head   *node[T]
	tail   *node[T]
	length int64
}

type node[T any] struct {
	val  T
	next *node[T]
	prev *node[T]
}

func newNode[T any](val T) *node[T] {
	return &node[T]{
		val: val,
	}
}

func newLinkedList[T any]() *LinkedList[T] {
	return &LinkedList[T]{}
}

func (linkedList *LinkedList[T]) Add(val T) {
	node := newNode(val)
	if linkedList.head == nil && linkedList.tail == nil {
		linkedList.head = node
		linkedList.tail = node
		return
	}
	prev := linkedList.tail
	linkedList.tail.next = node
	linkedList.tail = linkedList.tail.next
	linkedList.tail.prev = prev
}

func (linkedList *LinkedList[T]) Pop() (val T) {
	if linkedList.head == nil {
		return
	}
	val = linkedList.head.val
	if linkedList.head == linkedList.tail {
		linkedList.head, linkedList.tail = nil, nil
	} else {
		linkedList.head = linkedList.head.next
	}
	return val
}

func (linkedListp *LinkedList[T]) AddLength(num int64) {
	atomic.AddInt64(&linkedListp.length, num)
}

func (linkedListp *LinkedList[T]) GetLength(num int64) int64 {
	return atomic.LoadInt64(&linkedListp.length)
}
