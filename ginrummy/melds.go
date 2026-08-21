package ginrummy

import (
	"sort"
	"strconv"
)

// Meld is a set (3-4 same rank) or run (3+ consecutive, same suit) of cards.
type Meld struct {
	Kind  string `json:"kind"` // "set" or "run"
	Cards []Card `json:"-"`
	Codes []string `json:"cards"`
}

// Analysis is the best (minimal-deadwood) arrangement of a hand.
type Analysis struct {
	Deadwood int
	Melds    []Meld
	Unmatched []Card // leftover deadwood cards
}

// generateMelds returns every candidate meld in the hand as a bitmask over the
// hand slice indices, paired with the meld kind.
func generateMelds(hand []Card) (masks []uint32, kinds []string) {
	n := len(hand)

	// Sets: group indices by rank.
	byRank := map[int][]int{}
	for i, c := range hand {
		byRank[c.Rank()] = append(byRank[c.Rank()], i)
	}
	for _, idxs := range byRank {
		if len(idxs) >= 3 {
			// All 3-subsets and the full 4-set.
			for _, combo := range combinations(idxs, 3) {
				masks = append(masks, maskOf(combo))
				kinds = append(kinds, "set")
			}
			if len(idxs) == 4 {
				masks = append(masks, maskOf(idxs))
				kinds = append(kinds, "set")
			}
		}
	}

	// Runs: group indices by suit, sort by rank, find contiguous stretches.
	bySuit := map[int][]int{}
	for i := range hand {
		bySuit[hand[i].Suit()] = append(bySuit[hand[i].Suit()], i)
	}
	for _, idxs := range bySuit {
		sort.Slice(idxs, func(a, b int) bool { return hand[idxs[a]].Rank() < hand[idxs[b]].Rank() })
		// Walk contiguous runs of consecutive ranks (no duplicate ranks within a suit).
		start := 0
		for start < len(idxs) {
			end := start
			for end+1 < len(idxs) && hand[idxs[end+1]].Rank() == hand[idxs[end]].Rank()+1 {
				end++
			}
			run := idxs[start : end+1]
			// Every contiguous sub-run of length >= 3.
			for i := 0; i < len(run); i++ {
				for j := i + 2; j < len(run); j++ {
					masks = append(masks, maskOf(run[i:j+1]))
					kinds = append(kinds, "run")
				}
			}
			start = end + 1
		}
	}
	_ = n
	return
}

func maskOf(idxs []int) uint32 {
	var m uint32
	for _, i := range idxs {
		m |= 1 << uint(i)
	}
	return m
}

func combinations(items []int, k int) [][]int {
	var out [][]int
	var rec func(start int, cur []int)
	rec = func(start int, cur []int) {
		if len(cur) == k {
			cp := make([]int, k)
			copy(cp, cur)
			out = append(out, cp)
			return
		}
		for i := start; i < len(items); i++ {
			rec(i+1, append(cur, items[i]))
		}
	}
	rec(0, nil)
	return out
}

// Analyze finds the arrangement of melds minimizing total deadwood.
func Analyze(hand []Card) Analysis {
	masks, kinds := generateMelds(hand)

	full := uint32(0)
	for i := range hand {
		full |= 1 << uint(i)
	}

	type res struct {
		dw    int
		picks []int // indices into masks
	}
	memo := map[uint32]res{}

	var solve func(avail uint32) res
	solve = func(avail uint32) res {
		if avail == 0 {
			return res{0, nil}
		}
		if r, ok := memo[avail]; ok {
			return r
		}
		// Lowest available card index.
		low := 0
		for (avail>>uint(low))&1 == 0 {
			low++
		}
		// Option A: leave `low` as deadwood.
		best := solve(avail &^ (1 << uint(low)))
		best = res{best.dw + hand[low].Value(), best.picks}
		// Option B: use a meld containing `low` fully inside avail.
		for mi, m := range masks {
			if m&(1<<uint(low)) == 0 {
				continue
			}
			if m&avail != m {
				continue
			}
			sub := solve(avail &^ m)
			if sub.dw < best.dw {
				picks := append([]int{mi}, sub.picks...)
				best = res{sub.dw, picks}
			}
		}
		memo[avail] = best
		return best
	}

	r := solve(full)

	used := uint32(0)
	var melds []Meld
	for _, mi := range r.picks {
		m := masks[mi]
		used |= m
		var cs []Card
		for i := range hand {
			if m&(1<<uint(i)) != 0 {
				cs = append(cs, hand[i])
			}
		}
		sortCards(cs)
		melds = append(melds, Meld{Kind: kinds[mi], Cards: cs, Codes: codes(cs)})
	}
	var unmatched []Card
	for i := range hand {
		if used&(1<<uint(i)) == 0 {
			unmatched = append(unmatched, hand[i])
		}
	}
	sortCards(unmatched)
	return Analysis{Deadwood: r.dw, Melds: melds, Unmatched: unmatched}
}

func sortCards(cs []Card) {
	sort.Slice(cs, func(a, b int) bool {
		if cs[a].Suit() != cs[b].Suit() {
			return cs[a].Suit() < cs[b].Suit()
		}
		return cs[a].Rank() < cs[b].Rank()
	})
}

// canLayOff reports whether card c can be appended to meld m, and returns the
// resulting (grown) meld card list if so.
func canLayOff(m Meld, c Card) ([]Card, bool) {
	for _, x := range m.Cards {
		if x == c {
			return nil, false
		}
	}
	if m.Kind == "set" {
		if len(m.Cards) >= 4 {
			return nil, false
		}
		if c.Rank() == m.Cards[0].Rank() {
			return append(append([]Card{}, m.Cards...), c), true
		}
		return nil, false
	}
	// run: same suit, extends either end.
	suit := m.Cards[0].Suit()
	if c.Suit() != suit {
		return nil, false
	}
	lo := m.Cards[0].Rank()
	hi := m.Cards[len(m.Cards)-1].Rank()
	if c.Rank() == lo-1 || c.Rank() == hi+1 {
		grown := append(append([]Card{}, m.Cards...), c)
		sortCards(grown)
		return grown, true
	}
	return nil, false
}

// LayOff lays the given deadwood onto the melds (the knocker's melds, during
// scoring) in the arrangement that removes the most points, returning the
// reduced deadwood total and the cards laid off. It searches rather than being
// greedy: spending a card on one meld can block another card, so taking the
// highest-value lay-off first is not always best. With the knocker melding
// 7C 7D 7S and 4H 5H 6H, an opponent holding 7H and 8H must put the 7H on the
// run — 7H on the set strands the 8H for 8 points of deadwood that the rules
// allow them to shed.
func LayOff(deadwood []Card, melds []Meld) (remaining int, laidOff []Card) {
	dw := append([]Card{}, deadwood...)
	sortCards(dw)
	total := 0
	for _, c := range dw {
		total += c.Value()
	}
	if len(dw) == 0 || len(melds) == 0 {
		return total, nil
	}

	// work holds the melds as they grow during the search; kinds is fixed.
	work := make([][]Card, len(melds))
	kinds := make([]string, len(melds))
	for i, m := range melds {
		work[i] = append([]Card{}, m.Cards...)
		kinds[i] = m.Kind
	}

	avail := uint32(0)
	for i := range dw {
		avail |= 1 << uint(i)
	}

	bestVal := 0
	var bestPicks, cur []Card
	// seen prunes a state already explored having shed at least as much: the
	// remaining cards plus the current meld shapes fully determine the future.
	seen := map[string]int{}

	var rec func(avail uint32, val int)
	rec = func(avail uint32, val int) {
		if val > bestVal {
			bestVal = val
			bestPicks = append([]Card{}, cur...)
		}
		k := layoffStateKey(avail, work)
		if prev, ok := seen[k]; ok && prev >= val {
			return
		}
		seen[k] = val
		for i := range dw {
			if avail&(1<<uint(i)) == 0 {
				continue
			}
			for mi := range work {
				grown, ok := canLayOff(Meld{Kind: kinds[mi], Cards: work[mi]}, dw[i])
				if !ok {
					continue
				}
				before := work[mi]
				work[mi] = grown
				cur = append(cur, dw[i])
				rec(avail&^(1<<uint(i)), val+dw[i].Value())
				cur = cur[:len(cur)-1]
				work[mi] = before
			}
		}
	}
	rec(avail, 0)

	return total - bestVal, bestPicks
}

// layoffStateKey identifies a search state: which deadwood is still in hand,
// and how the melds have grown so far.
func layoffStateKey(avail uint32, work [][]Card) string {
	var b []byte
	b = strconv.AppendUint(b, uint64(avail), 36)
	for _, m := range work {
		b = append(b, '|')
		for _, c := range m {
			b = strconv.AppendInt(b, int64(c), 36)
			b = append(b, ',')
		}
	}
	return string(b)
}

// AnalyzeWithLayOff arranges a hand for the lowest deadwood when its holder may
// also lay cards off onto targets (the knocker's melds at scoring time). It
// picks the arrangement and the lay-offs together, because the arrangement with
// the least deadwood on its own can strand cards a slightly worse one would
// shed: holding 5S 5H 5D 5C 4S against a knocker's 6S 7S 8S, melding all four
// fives leaves the 4S stranded for 4 points, while melding only three frees the
// 5S to extend the run so the 4S follows it and nothing is left.
//
// The returned Analysis describes the chosen arrangement: Melds are the melds
// kept in hand, Unmatched and Deadwood are what survives after the lay-offs,
// and the second result is the cards laid off.
func AnalyzeWithLayOff(hand []Card, targets []Meld) (Analysis, []Card) {
	if len(hand) == 0 || len(targets) == 0 {
		return Analyze(hand), nil
	}
	masks, kinds := generateMelds(hand)

	// Walk every set of hand cards coverable by disjoint melds, keeping one
	// arrangement per covered set. Only the arrangements that cannot take
	// another meld are worth scoring: covering more can never leave more
	// deadwood, since a smaller leftover can always be laid off at least as far.
	arrangement := map[uint32][]int{0: nil}
	queue := []uint32{0}
	var settled []uint32
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		grew := false
		for mi, m := range masks {
			if m&cur != 0 {
				continue
			}
			grew = true
			next := cur | m
			if _, seen := arrangement[next]; !seen {
				arrangement[next] = append(append([]int{}, arrangement[cur]...), mi)
				queue = append(queue, next)
			}
		}
		if !grew {
			settled = append(settled, cur)
		}
	}

	bestDeadwood := -1
	var bestCover uint32
	var bestLaid []Card
	for _, cover := range settled {
		var left []Card
		for i := range hand {
			if cover&(1<<uint(i)) == 0 {
				left = append(left, hand[i])
			}
		}
		dw, laid := LayOff(left, targets)
		if bestDeadwood < 0 || dw < bestDeadwood {
			bestDeadwood, bestCover, bestLaid = dw, cover, laid
		}
	}

	laid := map[Card]bool{}
	for _, c := range bestLaid {
		laid[c] = true
	}
	var melds []Meld
	for _, mi := range arrangement[bestCover] {
		var cs []Card
		for i := range hand {
			if masks[mi]&(1<<uint(i)) != 0 {
				cs = append(cs, hand[i])
			}
		}
		sortCards(cs)
		melds = append(melds, Meld{Kind: kinds[mi], Cards: cs, Codes: codes(cs)})
	}
	var unmatched []Card
	for i := range hand {
		if bestCover&(1<<uint(i)) == 0 && !laid[hand[i]] {
			unmatched = append(unmatched, hand[i])
		}
	}
	sortCards(unmatched)
	return Analysis{Deadwood: bestDeadwood, Melds: melds, Unmatched: unmatched}, bestLaid
}
