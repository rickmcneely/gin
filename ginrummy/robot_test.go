package ginrummy

import "testing"

// TestRobotsPlayAFullGame guards against a robot stalling on a rule it cannot
// satisfy — the barred re-discard and the opening upcard offer both give a
// robot a move it must not make.
func TestRobotsPlayAFullGame(t *testing.T) {
	g := NewGame(1, []*Player{
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
	winner := 0
	for _, p := range g.Players {
		if p.UserID == g.WinnerID {
			winner = p.Score
		}
		if p.Boxes < 0 {
			t.Errorf("negative boxes for %s", p.Username)
		}
	}
	if winner < g.TargetScore {
		t.Errorf("winner finished on %d, want at least the %d target", winner, g.TargetScore)
	}
	t.Logf("game over after %d hands, %d robot steps", hands, steps)
}
