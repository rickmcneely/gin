package ginrummy

import "testing"

func twoHanded(t *testing.T) *Game {
	t.Helper()
	return NewGame(1, []*Player{{UserID: 1, Username: "A"}, {UserID: 2, Username: "B"}}, 100)
}

// The upcard is offered to the non-dealer first; if they decline, the dealer may
// take it, and taking it puts them on turn to discard.
func TestUpcardGoesToTheDealerWhenTheNonDealerPasses(t *testing.T) {
	g := twoHanded(t)
	up, _ := g.DiscardTop()

	if err := g.PassUpcard(2); err != nil { // non-dealer declines
		t.Fatalf("non-dealer pass: %v", err)
	}
	if g.Phase != PhaseUpcard || g.CurrentPlayer().UserID != 1 {
		t.Fatalf("offer should move to the dealer, got phase=%s turn=%d", g.Phase, g.CurrentPlayer().UserID)
	}
	c, err := g.TakeUpcard(1)
	if err != nil {
		t.Fatalf("dealer take: %v", err)
	}
	if c != up {
		t.Errorf("took %s, want the upcard %s", c.Code(), up.Code())
	}
	if g.Phase != PhaseDiscard || len(g.Players[0].Hand) != g.HandSize+1 {
		t.Fatalf("dealer should be discarding an 11-card hand, got phase=%s hand=%d", g.Phase, len(g.Players[0].Hand))
	}
	// Discarding passes the turn to the non-dealer, who plays a normal turn.
	other := otherThan(g.Players[0].Hand, c)
	if err := g.Discard(1, other, false); err != nil {
		t.Fatalf("dealer discard: %v", err)
	}
	if g.Phase != PhaseDraw || g.CurrentPlayer().UserID != 2 {
		t.Fatalf("turn should pass to player 2 to draw, got phase=%s turn=%d", g.Phase, g.CurrentPlayer().UserID)
	}
}

func TestUpcardPassedRoundForcesAStockDraw(t *testing.T) {
	g := twoHanded(t)
	if err := g.PassUpcard(2); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if err := g.PassUpcard(1); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if g.Phase != PhaseDraw || g.CurrentPlayer().UserID != 2 {
		t.Fatalf("phase=%s turn=%d, want player 2 on a forced stock draw", g.Phase, g.CurrentPlayer().UserID)
	}
	if _, err := g.Draw(2, true); err != ErrMustDrawStock {
		t.Errorf("draw from discard = %v, want ErrMustDrawStock", err)
	}
	if _, err := g.Draw(2, false); err != nil {
		t.Fatalf("stock draw: %v", err)
	}
	if g.StockOnly {
		t.Error("the restriction should lift once the opening draw is made")
	}
}

// A card taken from the discard pile cannot be discarded straight back.
func TestCannotDiscardTheCardJustTaken(t *testing.T) {
	g := twoHanded(t)
	taken, err := g.TakeUpcard(2)
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	if err := g.Discard(2, taken, false); err != ErrNoRedis {
		t.Fatalf("re-discard = %v, want ErrNoRedis", err)
	}
	other := otherThan(g.Players[1].Hand, taken)
	if err := g.Discard(2, other, false); err != nil {
		t.Fatalf("discarding a different card: %v", err)
	}
	// The restriction is per turn: it must not follow the card around.
	if g.TakenValid {
		t.Error("the taken-card restriction should be cleared at the end of the turn")
	}
}

func TestStockFloorCancelsTheHand(t *testing.T) {
	g := twoHanded(t)
	g.Phase = PhaseDraw
	g.StockOnly = false
	// Leave three cards: drawing the third-last and discarding ends the hand.
	g.Stock = g.Stock[:3]
	turn := g.TurnUserID()
	if _, err := g.Draw(turn, false); err != nil {
		t.Fatalf("draw: %v", err)
	}
	if len(g.Stock) != StockFloor {
		t.Fatalf("stock = %d, want %d", len(g.Stock), StockFloor)
	}
	hand := g.Players[g.Turn].Hand
	if err := g.Discard(turn, hand[0], false); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if g.Phase != PhaseRoundEnd {
		t.Fatalf("phase = %s, want the hand cancelled into roundEnd", g.Phase)
	}
	if !g.Washed {
		t.Error("the hand should be marked washed")
	}
	for _, p := range g.Players {
		if p.Score != 0 {
			t.Errorf("%s scored %d on a cancelled hand, want 0", p.Username, p.Score)
		}
	}
}

func TestCancelledHandKeepsTheSameDealer(t *testing.T) {
	g := twoHanded(t)
	dealer := g.DealerIdx
	g.endHandWashed()
	if err := g.NextHand(); err != nil {
		t.Fatalf("NextHand: %v", err)
	}
	if g.DealerIdx != dealer {
		t.Errorf("dealer moved to %d after a cancelled hand, want %d", g.DealerIdx, dealer)
	}
}

func TestWinnerOfAHandDealsTheNext(t *testing.T) {
	g := twoHanded(t)
	// Player 2 (index 1) takes the hand.
	g.LastResults = []HandResult{
		{UserID: 1, Username: "A", Points: 0},
		{UserID: 2, Username: "B", Points: 14},
	}
	g.Washed = false
	g.Phase = PhaseRoundEnd
	if err := g.NextHand(); err != nil {
		t.Fatalf("NextHand: %v", err)
	}
	if g.DealerIdx != 1 {
		t.Errorf("dealer = %d, want the hand's winner (index 1)", g.DealerIdx)
	}
}

// otherThan returns a card from hand that is not c.
func otherThan(hand []Card, c Card) Card {
	for _, x := range hand {
		if x != c {
			return x
		}
	}
	panic("hand has no other card")
}
