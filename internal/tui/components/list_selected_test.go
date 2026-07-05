package components

import "testing"

// SelectedIndex must never panic when Cursor is out of range — exported setters
// can write Cursor raw and a filter can shrink the visible rows beneath it.
func TestSelectedIndexClampsOutOfRangeCursor(t *testing.T) {
	l := List{Rows: []Row{{Left: "a"}, {Left: "b"}}, Cursor: 5}
	if got := l.SelectedIndex(); got != -1 {
		t.Fatalf("out-of-range cursor must return -1 (no panic), got %d", got)
	}
	l2 := List{Rows: []Row{{Left: "a"}}, Cursor: -1}
	if got := l2.SelectedIndex(); got != -1 {
		t.Fatalf("negative cursor must return -1, got %d", got)
	}
}
