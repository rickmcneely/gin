package rummy

import (
	"testing"

	gr "gin-server/ginrummy"
)

// TestRobotsPlayAFullGame guards against a robot stalling: the one-meld-per-turn
// limit and the barred re-discard both refuse a move a robot might try.
func TestRobotsPlayAFullGame(t *testing.T) {
	g := NewGame(1, []*gr.Player{
		{UserID: 1, Username: "R1", IsRobot: true},
		{UserID: 2, Username: "R2", IsRobot: true},
	}, 100)

	hands, steps := 0, 0
	for !g.GameOver() {
		if steps++; steps > 20000 {
			t.Fatalf("robots stalled after %d steps (hand %d, phase %s)", steps, g.HandNumber, g.Phase)
		}
		if _, _, _, acted := g.RobotStep(); acted {
			continue
		}
		if g.Phase != PhaseRoundEnd {
			t.Fatalf("robot could not act in phase %s (hand %d)", g.Phase, g.HandNumber)
		}
		if hands++; hands > 500 {
			t.Fatalf("game never reached %d points after %d hands", g.TargetScore, hands)
		}
		if err := g.NextHand(); err != nil {
			t.Fatalf("NextHand: %v", err)
		}
	}
	t.Logf("game over after %d hands, %d robot steps", hands, steps)
}
