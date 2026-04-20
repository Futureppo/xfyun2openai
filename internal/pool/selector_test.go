package pool

import (
	"reflect"
	"testing"
)

func TestSelectorRoundRobin(t *testing.T) {
	selector := NewSelector()

	order1 := selector.Order(3)
	selector.Advance(order1[0], 3)
	order2 := selector.Order(3)
	selector.Advance(order2[0], 3)
	order3 := selector.Order(3)

	if !reflect.DeepEqual(order1, []int{0, 1, 2}) {
		t.Fatalf("order1 = %v", order1)
	}
	if !reflect.DeepEqual(order2, []int{1, 2, 0}) {
		t.Fatalf("order2 = %v", order2)
	}
	if !reflect.DeepEqual(order3, []int{2, 0, 1}) {
		t.Fatalf("order3 = %v", order3)
	}
}

func TestSelectorOrderEmpty(t *testing.T) {
	selector := NewSelector()
	if got := selector.Order(0); got != nil {
		t.Fatalf("expected nil order, got %v", got)
	}
}
