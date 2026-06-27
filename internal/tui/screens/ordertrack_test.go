package screens

import "testing"

func TestStageFromStatus(t *testing.T) {
	cases := []struct {
		in            string
		stage         int
		delivered, ok bool
	}{
		{"Out for delivery", 2, false, true},
		{"on the way", 2, false, true},
		{"delivered", 3, true, true},
		{"Order delivered successfully", 3, true, true},
		{"preparing", 1, false, true},
		{"processing", 1, false, true},
		{"order confirmed", 0, false, true},
		{"No tracking information found for order 1", 0, false, false},
		{"whatever martian text", 0, false, false},
	}
	for _, c := range cases {
		s, d, ok := StageFromStatus(c.in)
		if s != c.stage || d != c.delivered || ok != c.ok {
			t.Errorf("%q -> (%d,%v,%v) want (%d,%v,%v)", c.in, s, d, ok, c.stage, c.delivered, c.ok)
		}
	}
}

func TestTrackProgressByTime(t *testing.T) {
	placed := int64(1_000_000)
	at := func(min int) int64 { return placed + int64(min)*60 }
	if st := TrackProgressByTime(placed, 30, 40, at(0)); st.Stage != 0 || !st.Estimated {
		t.Fatalf("t=0 %+v", st)
	}
	if st := TrackProgressByTime(placed, 30, 40, at(25)); st.Stage != 2 {
		t.Fatalf("t=25 (>55%% of 40) %+v", st)
	}
	if st := TrackProgressByTime(placed, 30, 40, at(55)); !st.Delivered {
		t.Fatalf("t=55 (>40+10 grace) must be delivered %+v", st)
	}
}
