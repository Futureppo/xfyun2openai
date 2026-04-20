package pool

import "sync"

type Selector struct {
	mu   sync.Mutex
	next int
}

func NewSelector() *Selector {
	return &Selector{}
}

func (s *Selector) Order(size int) []int {
	if size <= 0 {
		return nil
	}

	s.mu.Lock()
	start := s.next % size
	s.mu.Unlock()

	order := make([]int, 0, size)
	for i := 0; i < size; i++ {
		order = append(order, (start+i)%size)
	}

	return order
}

func (s *Selector) Advance(selectedIndex, size int) {
	if size <= 0 {
		return
	}

	s.mu.Lock()
	s.next = (selectedIndex + 1) % size
	s.mu.Unlock()
}
