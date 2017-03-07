package useranalytics

import (
	"testing"
)

func TestStack(t *testing.T) {

	luckyNumber := 11
	stack := NewStack(luckyNumber - 1)

	// push 1 through 11
	for i := 1; i <= luckyNumber; i++ {
		stack.Push(i)
		if x, exists := stack.Peek(); x.(int) != i || !exists {
			t.Errorf("Expected %d to be on top", i)
		}
	}

	// stack should max out at 10
	if stack.Len() != (luckyNumber - 1) {
		t.Errorf("Expected %d but got %d", (luckyNumber - 1), stack.Len())
	}

	items := stack.AsList()
	if len(items) != stack.max {
		t.Errorf("Expected %d but got %d", stack.max, len(items))
	}
	if items[0].(int) != luckyNumber {
		t.Errorf("Expected %d but got %d", luckyNumber, items[0].(int))
	}
	if items[len(items)-1].(int) != (luckyNumber-stack.Max())+1 {
		t.Errorf("Expected %d but got %d", luckyNumber, items[0].(int))
	}

	// expect to see 11 through 2, because the first one got dropped
	for i := luckyNumber; i > (luckyNumber - stack.Max()); i-- {
		if x := stack.Pop().(int); x != i {
			t.Errorf("Expected %d but got %d", i, x)
		}
	}

	// nothing should be left after popping all items from the stack
	if item, exists := stack.Peek(); exists {
		t.Errorf("Expected stack to be empty but found %#v", item)
	}

	if item := stack.PopLast(); item != nil {
		t.Errorf("Expected stack to be empty but found %#v", item)
	}

}
