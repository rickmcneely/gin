package ginrummy

import "testing"

// card builds a Card from a rank index (0=Ace..12=King) and suit index
// (0=Clubs, 1=Diamonds, 2=Hearts, 3=Spades).
func card(rank, suit int) Card { return Card(rank*4 + suit) }

func TestLayOffPrefersTheMeldThatKeepsOthersAlive(t *testing.T) {
	// Knocker melds 7C 7D 7S and 4H 5H 6H. The opponent holds 7H and 8H.
	// Laying the 7H on the set of sevens strands the 8H; laying it on the run
	// extends it to 4H-5H-6H-7H, so the 8H goes off too and nothing is left.
	set := Meld{Kind: "set", Cards: []Card{card(6, 0), card(6, 1), card(6, 3)}}
	run := Meld{Kind: "run", Cards: []Card{card(3, 2), card(4, 2), card(5, 2)}}
	dead := []Card{card(6, 2), card(7, 2)} // 7H 8H

	rem, laid := LayOff(dead, []Meld{set, run})
	if rem != 0 {
		t.Errorf("remaining = %d, want 0 (laid off %v)", rem, codes(laid))
	}
	if len(laid) != 2 {
		t.Errorf("laid off %v, want both 7H and 8H", codes(laid))
	}
}

func TestLayOffLeavesUnrelatedDeadwood(t *testing.T) {
	// KS has nowhere to go; the 7H still extends the run.
	run := Meld{Kind: "run", Cards: []Card{card(3, 2), card(4, 2), card(5, 2)}}
	dead := []Card{card(6, 2), card(12, 3)} // 7H, KS

	rem, laid := LayOff(dead, []Meld{run})
	if rem != 10 {
		t.Errorf("remaining = %d, want 10 (the KS)", rem)
	}
	if len(laid) != 1 || laid[0] != card(6, 2) {
		t.Errorf("laid off %v, want just 7H", codes(laid))
	}
}

func TestLayOffFillsASetOnlyToFour(t *testing.T) {
	// A set of three takes the fourth seven; a second seven does not exist, so
	// the 8C stays as deadwood.
	set := Meld{Kind: "set", Cards: []Card{card(6, 0), card(6, 1), card(6, 3)}}
	dead := []Card{card(6, 2), card(7, 0)} // 7H, 8C

	rem, laid := LayOff(dead, []Meld{set})
	if rem != 8 {
		t.Errorf("remaining = %d, want 8 (the 8C)", rem)
	}
	if len(laid) != 1 || laid[0] != card(6, 2) {
		t.Errorf("laid off %v, want just 7H", codes(laid))
	}
}

func TestLayOffNoMeldsOrNoDeadwood(t *testing.T) {
	if rem, laid := LayOff([]Card{card(12, 3)}, nil); rem != 10 || laid != nil {
		t.Errorf("no melds: got (%d, %v), want (10, nil)", rem, codes(laid))
	}
	if rem, laid := LayOff(nil, []Meld{{Kind: "set", Cards: []Card{card(6, 0), card(6, 1), card(6, 3)}}}); rem != 0 || laid != nil {
		t.Errorf("no deadwood: got (%d, %v), want (0, nil)", rem, codes(laid))
	}
}

func TestAnalyzeWithLayOffTradesAMeldForLayOffs(t *testing.T) {
	// 5S 5H 5D 5C 4S against a knocker melding 6S 7S 8S. Melding all four fives
	// is the best arrangement on its own (4 deadwood: the stranded 4S), but
	// melding three frees the 5S to extend the run, and the 4S follows it.
	hand := []Card{card(4, 3), card(4, 2), card(4, 1), card(4, 0), card(3, 3)}
	targets := []Meld{{Kind: "run", Cards: []Card{card(5, 3), card(6, 3), card(7, 3)}}}

	if base := Analyze(hand); base.Deadwood != 4 {
		t.Fatalf("premise changed: Analyze alone gives %d deadwood, want 4", base.Deadwood)
	}
	a, laid := AnalyzeWithLayOff(hand, targets)
	if a.Deadwood != 0 {
		t.Errorf("deadwood = %d, want 0 (melds %d, laid off %v, left %v)",
			a.Deadwood, len(a.Melds), codes(laid), codes(a.Unmatched))
	}
	if len(a.Melds) != 1 || len(a.Melds[0].Cards) != 3 {
		t.Errorf("want one meld of three fives, got %d melds", len(a.Melds))
	}
	if len(laid) != 2 {
		t.Errorf("laid off %v, want the 5S and the 4S", codes(laid))
	}
	if len(a.Unmatched) != 0 {
		t.Errorf("unmatched = %v, want nothing left", codes(a.Unmatched))
	}
}

func TestAnalyzeWithLayOffMatchesAnalyzeWithNothingToLayOff(t *testing.T) {
	// KS QH 2C against melds it cannot touch: same answer as a plain Analyze.
	hand := []Card{card(12, 3), card(11, 2), card(1, 0)}
	targets := []Meld{{Kind: "set", Cards: []Card{card(6, 0), card(6, 1), card(6, 3)}}}

	base := Analyze(hand)
	a, laid := AnalyzeWithLayOff(hand, targets)
	if a.Deadwood != base.Deadwood || len(laid) != 0 {
		t.Errorf("got deadwood %d laid %v, want %d and nothing laid off", a.Deadwood, codes(laid), base.Deadwood)
	}
}

func TestAnalyzeWithLayOffNoTargets(t *testing.T) {
	hand := []Card{card(12, 3), card(4, 3), card(3, 3)}
	a, laid := AnalyzeWithLayOff(hand, nil)
	if base := Analyze(hand); a.Deadwood != base.Deadwood || laid != nil {
		t.Errorf("got %d laid %v, want %d and nil", a.Deadwood, codes(laid), base.Deadwood)
	}
}
