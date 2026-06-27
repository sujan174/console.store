package screens

import (
	"fmt"
	"strings"
)

// TrackStages is the 4-step delivery timeline.
var TrackStages = []string{"order confirmed", "preparing", "out for delivery", "delivered"}

// stageRules map a status phrase (by case-insensitive substring) to a stage.
// Order matters: "out for delivery" is checked before the "deliver"→delivered
// rule so it never mis-maps to delivered.
var stageRules = []struct {
	keys      []string
	stage     int
	delivered bool
}{
	{[]string{"out for delivery", "on the way", "picked", "dispatch", "rider"}, 2, false},
	{[]string{"arriv", "reach", "near"}, 2, false},
	{[]string{"deliver", "completed", "handed"}, 3, true},
	{[]string{"prepar", "process", "cook", "kitchen", "making"}, 1, false},
	{[]string{"confirm", "placed", "received", "accept"}, 0, false},
}

// StageFromStatus maps a live status phrase to a stage. ok=false when nothing
// matches (caller falls back to time-driven); "no tracking information" is
// explicitly ok=false.
func StageFromStatus(status string) (int, bool, bool) {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" || strings.Contains(s, "no tracking information") {
		return 0, false, false
	}
	for _, r := range stageRules {
		for _, k := range r.keys {
			if strings.Contains(s, k) {
				return r.stage, r.delivered, true
			}
		}
	}
	return 0, false, false
}

// TrackState holds the computed delivery progress.
type TrackState struct {
	Stage     int
	Delivered bool
	ETAText   string
	Estimated bool
}

// TrackProgressByTime is the fallback engine: stage by elapsed vs etaHi.
func TrackProgressByTime(placedAt int64, etaLo, etaHi int, nowUnix int64) TrackState {
	if etaHi <= 0 {
		etaHi = 45
	}
	elapsedMin := int((nowUnix - placedAt) / 60)
	if elapsedMin < 0 {
		elapsedMin = 0
	}
	const grace = 10
	if elapsedMin >= etaHi+grace {
		return TrackState{Stage: 3, Delivered: true, ETAText: "est. delivered", Estimated: true}
	}
	frac := float64(elapsedMin) / float64(etaHi)
	stage := 0
	switch {
	case frac > 0.90:
		stage = 2
	case frac > 0.55:
		stage = 2
	case frac > 0.10:
		stage = 1
	}
	remain := etaHi - elapsedMin
	if remain < 0 {
		remain = 0
	}
	eta := fmt.Sprintf("~%d min (est.)", remain)
	if remain <= 3 {
		eta = "arriving (est.)"
	}
	return TrackState{Stage: stage, ETAText: eta, Estimated: true}
}
