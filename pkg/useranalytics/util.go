package useranalytics

//import "fmt"

type Stack struct {
	top  *Element
	size int
	max  int
}

type Element struct {
	value interface{}
	next  *Element
}

func NewStack(max int) *Stack {
	return &Stack{max: max}
}

// Return the stack's length
func (s *Stack) Len() int {
	return s.size
}

// Return the stack's max
func (s *Stack) Max() int {
	return s.max
}

// Push a new element onto the stack
func (s *Stack) Push(value interface{}) {
	if s.size+1 > s.max {
		if last := s.PopLast(); last == nil {
			panic("Unexpected nil in stack")
		}
	}
	s.top = &Element{value, s.top}
	s.size++
}

//Peek returns a top without removing it from list
func (s *Stack) Peek() (value interface{}, exists bool) {
	exists = false
	if s.size > 0 {
		value = s.top.value
		exists = true
	}

	return
}

// Remove the top element from the stack and return it's value
// If the stack is empty, return nil
func (s *Stack) Pop() (value interface{}) {
	if s.size > 0 {
		value, s.top = s.top.value, s.top.next
		s.size--
		return
	}
	return nil
}

func (s *Stack) PopLast() (value interface{}) {
	if last := s.popLast(s.top); last != nil {
		return last.value
	}
	return nil
}

func (s *Stack) popLast(elem *Element) *Element {
	if elem == nil {
		return nil
	}
	// not last because it has next and a grandchild
	if elem.next != nil && elem.next.next != nil {
		return s.popLast(elem.next)
	}

	// current elem is second from bottom, as next elem has no child
	if elem.next != nil && elem.next.next == nil {
		last := elem.next
		// make current elem bottom of stack by removing its next element
		elem.next = nil
		s.size--
		return last
	}
	return nil
}

func (s *Stack) AsList() []interface{} {
	items := make([]interface{}, s.size)
	s.fillList(s.top, items, 0)
	return items
}

func (s *Stack) fillList(elem *Element, items []interface{}, depth int) {
	if elem == nil {
		return
	}

	items[depth] = elem.value

	if elem.next != nil {
		depth++
		s.fillList(elem.next, items, depth)
	}
}
