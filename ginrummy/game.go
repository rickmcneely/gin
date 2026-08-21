package ginrummy

import (
	"errors"
	"sync"
)

const (
	PhaseUpcard   = "upcard"   // opening: current player may take the face-up card or pass
	PhaseDraw     = "draw"     // current player must draw
	PhaseDiscard  = "discard"  // current player must discard (and may knock)
	PhaseRoundEnd = "roundEnd" // hand finished, results available
	PhaseGameOver = "gameOver" // someone reached the target score
)

const KnockThreshold = 10

// Scoring bonuses, following pagat's Gin Rummy scoring. Box (line) bonuses and
// the game bonus are awarded on top of hand points and do not count towards
// TargetScore — only the deadwood differences do.
const (
	GinBonus      = 20  // going gin, on top of the opponent's deadwood
	UndercutBonus = 10  // undercutting the knocker, on top of the difference
	BoxBonus      = 20  // per hand won, added at the end of the game
	GameBonus     = 100 // for reaching the target score first
	ShutoutBonus  = 200 // ... when no opponent scored a single point
)

// StockFloor is how many cards must remain in the stock for play to continue:
// when a player discards without knocking and only this many are left, the hand
// is cancelled and dealt again by the same dealer.
const StockFloor = 2

var (
	ErrNotYourTurn  = errors.New("it's not your turn yet — wait for the other players to act")
	ErrWrongPhase   = errors.New("you can't do that right now")
	ErrEmptyStock   = errors.New("the stock pile ran out, so this hand is a draw")
	ErrNoCard       = errors.New("you don't have that card in your hand")
	ErrCannotKnock  = errors.New("you can't knock — your deadwood must be 10 or less")
	ErrEmptyDiscard = errors.New("the discard pile is empty — draw from the stock instead")
	ErrMustDrawStock = errors.New("the upcard was passed round, so this draw must come from the stock")
	ErrNoRedis       = errors.New("you can't discard the card you just took from the discard pile")
)

// Player is a seat at the table.
type Player struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	IsRobot  bool   `json:"is_robot"`
	Hand     []Card `json:"-"`
	Score    int    `json:"score"`  // hand points; this is what reaches TargetScore
	Boxes    int    `json:"boxes"`  // hands won, each worth BoxBonus at the end
	Bonus    int    `json:"bonus"`  // game bonus, awarded when the game is over
	Connected bool  `json:"connected"`
}

// Total is the player's final score: hand points, plus a box bonus for each
// hand won, plus the game bonus if they won the game.
func (p *Player) Total() int { return p.Score + BoxBonus*p.Boxes + p.Bonus }

// HandResult captures one player's standing when a hand ends.
type HandResult struct {
	UserID   int      `json:"user_id"`
	Username string   `json:"username"`
	Melds    []Meld   `json:"melds"`
	Deadwood int      `json:"deadwood"`
	Points   int      `json:"points"`     // points scored this hand
	IsKnocker bool    `json:"is_knocker"`
	Gin      bool     `json:"gin"`
	Undercut bool     `json:"undercut"`
	Boxes    int      `json:"boxes"`    // the winner's running count of hands won
	HandCodes []string `json:"hand"`
	LaidOff  []string `json:"laid_off"` // cards shed onto the knocker's melds
}

// Game is a single gin rummy match (best-of to TargetScore).
type Game struct {
	ID          int       `json:"id"`
	Players     []*Player `json:"players"`
	Stock       []Card    `json:"-"`
	DiscardPile []Card    `json:"-"`
	Turn        int       `json:"turn"`         // index into Players
	Phase       string    `json:"phase"`
	HandNumber  int       `json:"hand_number"`
	DealerIdx   int       `json:"dealer_idx"`
	TargetScore int       `json:"target_score"`
	HandSize    int       `json:"hand_size"`
	WinnerID    int       `json:"winner_id"` // 0 until game over
	LastResults []HandResult `json:"-"`
	LastDrawFrom string   `json:"-"` // "stock" or "discard" of the most recent draw, for UI hints
	UpcardPasses int      `json:"-"` // how many players have declined the opening upcard
	StockOnly    bool     `json:"-"` // the upcard went round untaken: this draw must be from the stock
	TakenCard    Card     `json:"-"` // card taken from the discard this turn — it cannot be discarded back
	TakenValid   bool     `json:"-"`
	Washed       bool     `json:"-"` // the last hand was cancelled, so the same dealer deals again
	mu          sync.Mutex
}

// NewGame creates and deals the first hand. players must have UserID/Username/IsRobot set.
func NewGame(id int, players []*Player, target int) *Game {
	g := &Game{
		ID:          id,
		Players:     players,
		TargetScore: target,
		DealerIdx:   0,
	}
	if len(players) >= 3 {
		g.HandSize = 7
	} else {
		g.HandSize = 10
	}
	g.deal()
	return g
}

// Lock/Unlock expose the game mutex for the caller (handlers serialize a game).
func (g *Game) Lock()   { g.mu.Lock() }
func (g *Game) Unlock() { g.mu.Unlock() }

func (g *Game) deal() {
	deck := NewDeck()
	Shuffle(deck)
	for _, p := range g.Players {
		p.Hand = nil
	}
	pos := 0
	for r := 0; r < g.HandSize; r++ {
		for _, p := range g.Players {
			p.Hand = append(p.Hand, deck[pos])
			pos++
		}
	}
	for _, p := range g.Players {
		sortCards(p.Hand)
	}
	g.DiscardPile = []Card{deck[pos]}
	pos++
	g.Stock = deck[pos:]
	g.Turn = (g.DealerIdx + 1) % len(g.Players) // the eldest hand is offered the upcard first
	g.Phase = PhaseUpcard
	g.HandNumber++
	g.LastDrawFrom = ""
	g.UpcardPasses = 0
	g.StockOnly = false
	g.clearTaken()
}

// clearTaken forgets the card taken from the discard pile, which is only barred
// from being discarded during the turn it was taken.
func (g *Game) clearTaken() {
	g.TakenCard, g.TakenValid = 0, false
}

// TakeUpcard is the opening option: the player being offered the face-up card
// takes it into their hand and continues the turn by discarding.
func (g *Game) TakeUpcard(userID int) (Card, error) {
	if g.playerIndex(userID) != g.Turn {
		return 0, ErrNotYourTurn
	}
	if g.Phase != PhaseUpcard {
		return 0, ErrWrongPhase
	}
	if len(g.DiscardPile) == 0 {
		return 0, ErrEmptyDiscard
	}
	c := g.DiscardPile[len(g.DiscardPile)-1]
	g.DiscardPile = g.DiscardPile[:len(g.DiscardPile)-1]
	p := g.Players[g.Turn]
	p.Hand = append(p.Hand, c)
	sortCards(p.Hand)
	g.TakenCard, g.TakenValid = c, true
	g.LastDrawFrom = "discard"
	g.Phase = PhaseDiscard
	return c, nil
}

// PassUpcard declines the opening upcard and offers it to the next player. Once
// everyone has passed, the eldest hand draws from the stock to start the hand.
func (g *Game) PassUpcard(userID int) error {
	if g.playerIndex(userID) != g.Turn {
		return ErrNotYourTurn
	}
	if g.Phase != PhaseUpcard {
		return ErrWrongPhase
	}
	g.UpcardPasses++
	if g.UpcardPasses >= len(g.Players) {
		// Nobody wanted it: the eldest hand starts by drawing from the stock.
		g.Turn = (g.DealerIdx + 1) % len(g.Players)
		g.Phase = PhaseDraw
		g.StockOnly = true
		return nil
	}
	g.Turn = (g.Turn + 1) % len(g.Players)
	return nil
}

func (g *Game) playerIndex(userID int) int {
	for i, p := range g.Players {
		if p.UserID == userID {
			return i
		}
	}
	return -1
}

// CurrentPlayer returns the player whose turn it is (nil if round/game over).
func (g *Game) CurrentPlayer() *Player {
	if g.Phase != PhaseUpcard && g.Phase != PhaseDraw && g.Phase != PhaseDiscard {
		return nil
	}
	return g.Players[g.Turn]
}

// Draw takes the top of the stock or the discard pile for the current player.
func (g *Game) Draw(userID int, fromDiscard bool) (Card, error) {
	idx := g.playerIndex(userID)
	if idx != g.Turn {
		return 0, ErrNotYourTurn
	}
	if g.Phase != PhaseDraw {
		return 0, ErrWrongPhase
	}
	p := g.Players[idx]
	var c Card
	if fromDiscard {
		if g.StockOnly {
			return 0, ErrMustDrawStock
		}
		if len(g.DiscardPile) == 0 {
			return 0, ErrEmptyDiscard
		}
		c = g.DiscardPile[len(g.DiscardPile)-1]
		g.DiscardPile = g.DiscardPile[:len(g.DiscardPile)-1]
		g.LastDrawFrom = "discard"
		g.TakenCard, g.TakenValid = c, true
	} else {
		if len(g.Stock) == 0 {
			// Stock exhausted with no knock: the hand is a draw.
			g.endHandWashed()
			return 0, ErrEmptyStock
		}
		c = g.Stock[len(g.Stock)-1]
		g.Stock = g.Stock[:len(g.Stock)-1]
		g.LastDrawFrom = "stock"
		g.clearTaken()
	}
	g.StockOnly = false
	p.Hand = append(p.Hand, c)
	sortCards(p.Hand)
	g.Phase = PhaseDiscard
	return c, nil
}

// Discard plays a card from the current player's hand. If knock is true the
// player attempts to knock/gin; the hand then ends and is scored.
func (g *Game) Discard(userID int, card Card, knock bool) error {
	idx := g.playerIndex(userID)
	if idx != g.Turn {
		return ErrNotYourTurn
	}
	if g.Phase != PhaseDiscard {
		return ErrWrongPhase
	}
	p := g.Players[idx]
	pos := -1
	for i, c := range p.Hand {
		if c == card {
			pos = i
			break
		}
	}
	if pos < 0 {
		return ErrNoCard
	}
	if g.TakenValid && card == g.TakenCard {
		return ErrNoRedis
	}

	if knock {
		// Evaluate the hand WITHOUT the discard.
		remaining := append([]Card{}, p.Hand[:pos]...)
		remaining = append(remaining, p.Hand[pos+1:]...)
		a := Analyze(remaining)
		if a.Deadwood > KnockThreshold {
			return ErrCannotKnock
		}
	}

	// Remove the discard from hand and place it on the pile.
	p.Hand = append(p.Hand[:pos], p.Hand[pos+1:]...)
	g.DiscardPile = append(g.DiscardPile, card)

	if knock {
		g.scoreHand(idx)
		return nil
	}

	g.clearTaken()

	// The hand dies once the stock is down to its floor and nobody has knocked.
	if len(g.Stock) <= StockFloor {
		g.endHandWashed()
		return nil
	}

	// Advance to the next player.
	g.Turn = (g.Turn + 1) % len(g.Players)
	g.Phase = PhaseDraw
	g.LastDrawFrom = ""
	return nil
}

func (g *Game) endHandWashed() {
	g.Washed = true
	g.LastResults = nil
	for _, p := range g.Players {
		a := Analyze(p.Hand)
		g.LastResults = append(g.LastResults, HandResult{
			UserID: p.UserID, Username: p.Username,
			Melds: a.Melds, Deadwood: a.Deadwood, Points: 0,
			HandCodes: codes(p.Hand),
		})
	}
	g.afterHand()
}

// scoreHand computes results for a knock/gin by player at knockerIdx.
func (g *Game) scoreHand(knockerIdx int) {
	g.Washed = false
	knocker := g.Players[knockerIdx]
	ka := Analyze(knocker.Hand)
	gin := ka.Deadwood == 0

	results := make([]HandResult, len(g.Players))
	for i, p := range g.Players {
		a := Analyze(p.Hand)
		results[i] = HandResult{
			UserID: p.UserID, Username: p.Username,
			Melds: a.Melds, Deadwood: a.Deadwood,
			HandCodes: codes(p.Hand),
		}
	}
	results[knockerIdx].IsKnocker = true
	results[knockerIdx].Gin = gin

	knockerGain := 0
	for i, p := range g.Players {
		if i == knockerIdx {
			continue
		}
		oppDead := results[i].Deadwood
		if !gin {
			// Opponents may lay off onto the knocker's melds, and may arrange
			// their own hand to suit the lay-offs — the arrangement with the
			// least deadwood on its own can strand cards they could shed.
			oa, laid := AnalyzeWithLayOff(p.Hand, ka.Melds)
			oppDead = oa.Deadwood
			results[i].Deadwood = oppDead
			results[i].Melds = oa.Melds
			results[i].LaidOff = codes(laid)
		}
		if gin {
			// Gin: knocker collects each opponent's full deadwood; no undercut.
			knockerGain += oppDead
		} else if oppDead > ka.Deadwood {
			knockerGain += oppDead - ka.Deadwood
		} else {
			// Undercut: this opponent scores the difference plus the bonus.
			pts := (ka.Deadwood - oppDead) + UndercutBonus
			results[i].Points += pts
			results[i].Undercut = true
			g.Players[i].Score += pts
		}
	}
	if gin {
		knockerGain += GinBonus // once, however many opponents there are
	}
	results[knockerIdx].Points = knockerGain
	knocker.Score += knockerGain

	// A box for each player who took points off this hand.
	for i := range g.Players {
		if results[i].Points > 0 {
			g.Players[i].Boxes++
			results[i].Boxes = g.Players[i].Boxes
		}
	}

	g.LastResults = results
	g.afterHand()
}

func (g *Game) afterHand() {
	// Check for a match winner.
	winner, best := 0, -1
	reached := false
	for _, p := range g.Players {
		if p.Score >= g.TargetScore {
			reached = true
		}
		if p.Score > best {
			best = p.Score
			winner = p.UserID
		}
	}
	if reached {
		g.WinnerID = winner
		g.awardGameBonus(winner)
		g.Phase = PhaseGameOver
		return
	}
	g.Phase = PhaseRoundEnd
}

// awardGameBonus gives the game winner their bonus — doubled when no opponent
// scored a single point over the whole game.
func (g *Game) awardGameBonus(winnerID int) {
	shutout := true
	for _, p := range g.Players {
		if p.UserID != winnerID && p.Score > 0 {
			shutout = false
			break
		}
	}
	for _, p := range g.Players {
		if p.UserID == winnerID {
			if shutout {
				p.Bonus = ShutoutBonus
			} else {
				p.Bonus = GameBonus
			}
			return
		}
	}
}

// NextHand deals a fresh hand after a round has ended, rotating the dealer.
func (g *Game) NextHand() error {
	if g.Phase != PhaseRoundEnd {
		return ErrWrongPhase
	}
	// The winner of a hand deals the next one; a cancelled hand is dealt again
	// by the same dealer.
	if !g.Washed {
		if w := g.lastHandWinner(); w >= 0 {
			g.DealerIdx = w
		}
	}
	g.deal()
	return nil
}

// lastHandWinner is the index of the player who scored the most points in the
// hand just finished, or -1 if nobody scored.
func (g *Game) lastHandWinner() int {
	best, idx := 0, -1
	for _, r := range g.LastResults {
		if r.Points > best {
			if i := g.playerIndex(r.UserID); i >= 0 {
				best, idx = r.Points, i
			}
		}
	}
	return idx
}

// DiscardTop returns the current top discard card and whether one exists.
func (g *Game) DiscardTop() (Card, bool) {
	if len(g.DiscardPile) == 0 {
		return 0, false
	}
	return g.DiscardPile[len(g.DiscardPile)-1], true
}
