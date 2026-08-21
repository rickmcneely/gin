package rummy

import (
	"testing"

	gr "gin-server/ginrummy"
)

func TestOnlyOneMeldPerTurn(t *testing.T) {
	g := newTestGame()
	g.laidBefore = map[int]bool{}
	g.Players[0].Hand = parse("4H", "5H", "6H", "7C", "7D", "7S", "KH")
	g.Players[1].Hand = parse("2C")

	if _, _, err := g.Meld(1, []string{"4H", "5H", "6H"}); err != nil {
		t.Fatalf("first meld: %v", err)
	}
	if _, _, err := g.Meld(1, []string{"7C", "7D", "7S"}); err != errOneMeld {
		t.Fatalf("second meld = %v, want errOneMeld", err)
	}
	// Discarding ends the turn, and the next turn allows a meld again.
	if _, _, err := g.Discard(1, "KH"); err != nil {
		t.Fatalf("discard: %v", err)
	}
	g.Turn = 0
	g.Phase = PhasePlay
	if _, _, err := g.Meld(1, []string{"7C", "7D", "7S"}); err != nil {
		t.Errorf("meld on the next turn: %v", err)
	}
}

func TestLayOffsAreNotLimitedByTheMeldAllowance(t *testing.T) {
	g := newTestGame()
	g.laidBefore = map[int]bool{}
	g.Table = []*TableMeld{tableMeld("3C", "4C", "5C"), tableMeld("7H", "8H", "9H")}
	g.Players[0].Hand = parse("2C", "6C", "TH", "KD")
	g.Players[1].Hand = parse("2D")

	for _, c := range []string{"2C", "6C", "TH"} {
		if _, _, err := g.Layoff(1, c, meldIndexFor(t, g, c)); err != nil {
			t.Fatalf("lay off %s: %v", c, err)
		}
	}
	if len(g.Players[0].Hand) != 1 {
		t.Errorf("hand = %v, want just the KD left", gr.Codes(g.Players[0].Hand))
	}
}

func TestGoingRummyOnlyCountsWhenNothingWasLaidEarlier(t *testing.T) {
	g := newTestGame()
	g.laidBefore = map[int]bool{}
	g.Players[0].Hand = parse("4H", "5H", "6H", "KH", "QS")
	g.Players[1].Hand = parse("KD", "KS") // 20 points

	// Turn one: meld and discard, still holding the QS, so the hand is not shed
	// in a single turn.
	if _, _, err := g.Meld(1, []string{"4H", "5H", "6H"}); err != nil {
		t.Fatalf("meld: %v", err)
	}
	if _, _, err := g.Discard(1, "KH"); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if !g.laidBefore[1] {
		t.Fatal("melding should be remembered past the end of the turn")
	}
	// Turn two: go out. No doubling, because the hand took more than one turn.
	g.Turn = 0
	g.Phase = PhasePlay
	g.Players[0].Hand = nil
	g.goOut(0)

	if g.LastResults[0].Rummy {
		t.Error("going out across two turns should not count as a rummy")
	}
	if g.Players[0].Score != 20 {
		t.Errorf("score = %d, want 20 undoubled", g.Players[0].Score)
	}
}

func TestCannotDiscardTheCardJustDrawn(t *testing.T) {
	g := newTestGame()
	g.laidBefore = map[int]bool{}
	g.Phase = PhaseDraw
	g.Players[0].Hand = parse("KH")
	g.Players[1].Hand = parse("2C")
	g.DiscardPile = parse("9S")

	c, err := g.Draw(1, true)
	if err != nil {
		t.Fatalf("draw from discard: %v", err)
	}
	if _, _, err := g.Discard(1, c.Code()); err != errNoRedis {
		t.Fatalf("re-discard = %v, want errNoRedis", err)
	}
	if _, _, err := g.Discard(1, "KH"); err != nil {
		t.Fatalf("discarding another card: %v", err)
	}
}

func TestStockIsRecycledWithoutShuffling(t *testing.T) {
	g := newTestGame()
	g.laidBefore = map[int]bool{}
	g.Phase = PhaseDraw
	g.Stock = nil
	g.progressed = true
	// Pile runs bottom to top: 2C was played first, KS is the current top.
	g.DiscardPile = parse("2C", "5D", "9H", "KS")
	g.Players[0].Hand = parse("4S")
	g.Players[1].Hand = parse("6S")

	c, err := g.Draw(1, false)
	if err != nil {
		t.Fatalf("draw after recycling: %v", err)
	}
	// Turning the pile over puts its bottom card on top of the new stock.
	if c.Code() != "2C" {
		t.Errorf("drew %s, want 2C — the pile should be turned over, not shuffled", c.Code())
	}
	if got := gr.Codes(g.Stock); len(got) != 2 || got[0] != "9H" || got[1] != "5D" {
		t.Errorf("stock = %v, want [9H 5D] (next draws 5D then 9H)", got)
	}
	if len(g.DiscardPile) != 1 || g.DiscardPile[0].Code() != "KS" {
		t.Errorf("discard = %v, want just the KS left as the top", gr.Codes(g.DiscardPile))
	}
}

// meldIndexFor finds the table meld that accepts the given card.
func meldIndexFor(t *testing.T, g *RummyGame, code string) int {
	t.Helper()
	card := parse(code)[0]
	for i, tm := range g.Table {
		if _, ok := gr.CanLayOff(tm.asMeld(), card); ok {
			return i
		}
	}
	t.Fatalf("no table meld accepts %s", code)
	return -1
}
