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

// SelectedIndex must resolve to the row actually under the cursor even when two
// rows are byte-identical — a value search would collapse both to the first
// (audit #3: two same-name/price dishes → + acted on the wrong SwiggyID).
func TestSelectedIndexDistinguishesIdenticalRows(t *testing.T) {
	dup := Row{Left: "Cold Brew", Right: "₹240"}
	l := List{Rows: []Row{dup, dup, {Left: "Latte"}}, Cursor: 1}
	if got := l.SelectedIndex(); got != 1 {
		t.Fatalf("cursor on the 2nd identical row must return index 1, got %d", got)
	}
}

// With a filter active, SelectedIndex must map back to the SOURCE index, not the
// visible position.
func TestSelectedIndexMapsThroughFilter(t *testing.T) {
	l := List{Rows: []Row{{Left: "Apple"}, {Left: "Banana"}, {Left: "Berry"}}}
	l.SetFilter("b") // visible: Banana(1), Berry(2)
	l.Cursor = 1     // 2nd visible = Berry = source index 2
	if got := l.SelectedIndex(); got != 2 {
		t.Fatalf("filtered cursor must map to source index 2 (Berry), got %d", got)
	}
}

// View must not panic when Cursor is past the end with a windowed viewport
// (audit #6: the windowed branch could push end past len(vis)).
func TestViewWindowedOutOfRangeCursorNoPanic(t *testing.T) {
	rows := make([]Row, 10)
	for i := range rows {
		rows[i] = Row{Left: string(rune('a' + i))}
	}
	l := List{Rows: rows, MaxRows: 4, Cursor: 99, Width: 40}
	_ = l.View() // must not panic
}
