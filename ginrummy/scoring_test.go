package ginrummy

import "testing"

// scored builds a two-handed game with hands set up for a known knock: player 1
// knocks with `knockerHand`, player 2 holds `oppHand`.
func scored(t *testing.T, knockerHand, oppHand []Card) *Game {
	t.Helper()
	g := NewGame(1, []*Player{{UserID: 1, Username: "A"}, {UserID: 2, Username: "B"}}, 100)
	g.Players[0].Hand = append([]Card{}, knockerHand...)
	g.Players[1].Hand = append([]Card{}, oppHand...)
	g.Turn = 0
	g.Phase = PhaseDiscard
	g.clearTaken()
	return g
}

func TestGinBonusIsTwenty(t *testing.T) {
	// Knocker: run 2C 3C 4C + set 9C 9D 9H, plus a spare KS to discard.
	knock := []Card{mk(1, 0), mk(2, 0), mk(3, 0), mk(8, 0), mk(8, 1), mk(8, 2), mk(12, 3)}
	// Opponent: KH QH JS 2S -> nothing melds, nothing lays off (10+10+10+2 = 32).
	opp := []Card{mk(12, 2), mk(11, 2), mk(10, 3), mk(1, 3)}
	g := scored(t, knock, opp)

	if err := g.Discard(1, mk(12, 3), true); err != nil {
		t.Fatalf("gin knock: %v", err)
	}
	res := g.LastResults[0]
	if !res.Gin {
		t.Fatalf("expected gin, got %+v", res)
	}
	if want := 32 + GinBonus; res.Points != want {
		t.Errorf("gin points = %d, want %d (32 deadwood + %d bonus)", res.Points, want, GinBonus)
	}
	if g.Players[0].Boxes != 1 {
		t.Errorf("boxes = %d, want 1 for the hand won", g.Players[0].Boxes)
	}
	if g.Players[1].Boxes != 0 {
		t.Errorf("loser took a box: %d", g.Players[1].Boxes)
	}
}

func TestUndercutBonusIsTen(t *testing.T) {
	// Knocker melds 2C 3C 4C and keeps 5S (deadwood 5), discarding the KS.
	knock := []Card{mk(1, 0), mk(2, 0), mk(3, 0), mk(4, 3), mk(12, 3)}
	// Opponent melds 9C 9D 9H and keeps 2S (deadwood 2) — under the knocker.
	opp := []Card{mk(8, 0), mk(8, 1), mk(8, 2), mk(1, 3)}
	g := scored(t, knock, opp)

	if err := g.Discard(1, mk(12, 3), true); err != nil {
		t.Fatalf("knock: %v", err)
	}
	oppRes := g.LastResults[1]
	if !oppRes.Undercut {
		t.Fatalf("expected an undercut, got %+v", g.LastResults)
	}
	if want := (5 - 2) + UndercutBonus; oppRes.Points != want {
		t.Errorf("undercut points = %d, want %d (difference 3 + %d bonus)", oppRes.Points, want, UndercutBonus)
	}
	if g.LastResults[0].Points != 0 {
		t.Errorf("knocker scored %d on an undercut, want 0", g.LastResults[0].Points)
	}
	if g.Players[1].Boxes != 1 || g.Players[0].Boxes != 0 {
		t.Errorf("boxes = %d/%d, want the undercutter to take the box", g.Players[0].Boxes, g.Players[1].Boxes)
	}
}

func TestGameBonusAndBoxesOnlyCountAtTheEnd(t *testing.T) {
	g := NewGame(1, []*Player{{UserID: 1, Username: "A"}, {UserID: 2, Username: "B"}}, 100)
	g.Players[0].Score = 96
	g.Players[0].Boxes = 4
	g.Players[1].Score = 40
	g.Players[1].Boxes = 3

	// Winning a hand worth 8 takes A past the target.
	g.LastResults = []HandResult{{UserID: 1, Points: 8}, {UserID: 2}}
	g.Players[0].Score += 8
	g.Players[0].Boxes++
	g.afterHand()

	if g.Phase != PhaseGameOver || g.WinnerID != 1 {
		t.Fatalf("phase=%s winner=%d, want the game over and won by 1", g.Phase, g.WinnerID)
	}
	if g.Players[0].Bonus != GameBonus {
		t.Errorf("game bonus = %d, want %d", g.Players[0].Bonus, GameBonus)
	}
	if want := 104 + BoxBonus*5 + GameBonus; g.Players[0].Total() != want {
		t.Errorf("winner total = %d, want %d", g.Players[0].Total(), want)
	}
	if want := 40 + BoxBonus*3; g.Players[1].Total() != want {
		t.Errorf("loser total = %d, want %d (boxes still count)", g.Players[1].Total(), want)
	}
}

func TestShutoutDoublesTheGameBonus(t *testing.T) {
	g := NewGame(1, []*Player{{UserID: 1, Username: "A"}, {UserID: 2, Username: "B"}}, 100)
	g.Players[0].Score = 100
	g.Players[1].Score = 0
	g.LastResults = []HandResult{{UserID: 1, Points: 12}, {UserID: 2}}
	g.afterHand()

	if g.Players[0].Bonus != ShutoutBonus {
		t.Errorf("bonus = %d, want %d for a shutout", g.Players[0].Bonus, ShutoutBonus)
	}
}
