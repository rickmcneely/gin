# Rules

Both games follow [pagat.com](https://www.pagat.com), the reference this project
treats as authoritative:

- Gin Rummy — <https://www.pagat.com/rummy/ginrummy.html>
- Standard (Basic) Rummy — <https://www.pagat.com/rummy/rummy.html>

Neither game has a governing body, so pagat's main text is the ruleset here and
its listed variations are only adopted where this file says so. Where the engine
departs from pagat, it is listed under **House rules** below — nowhere else.

Card values are the same in both games: aces 1, face cards 10, everything else
its spot value. Aces are low only, so `A-2-3` is a run and `Q-K-A` is not.

## Gin Rummy

**The deal.** Two players, ten cards each, dealt one at a time. The next card is
turned face up to start the discard pile and the rest becomes the stock. The
winner of a hand deals the next one.

**The opening.** The face-up card is offered before anyone draws. The non-dealer
may take it; if they decline it is offered to the dealer; if both decline, the
non-dealer opens by drawing from the stock and the discard pile is closed for
that draw. Whoever takes the upcard finishes the turn by discarding.

**A turn.** Draw one card, from the stock or the top of the discard pile, then
discard one. A card taken from the discard pile cannot be discarded back on the
same turn.

**Knocking.** Instead of an ordinary discard you may knock when the cards left
after the discard come to 10 points or less outside sets and runs. Knocking with
nothing left over is gin.

**Laying off.** After a knock — but not after gin — the opponent may extend the
knocker's sets and runs with their own unmatched cards. The engine arranges the
opponent's hand and their lay-offs together, choosing whichever combination
leaves them the least deadwood; a knocker never lays off on the opponent.

**End of the hand.** The hand is cancelled, with no score, once the stock is
down to two cards and the player who drew the third-last card discards without
knocking. A cancelled hand is dealt again by the same dealer.

**Scoring.**

| | |
|---|---|
| Knock | the difference between the two deadwood counts |
| Undercut (opponent's deadwood is equal or lower) | the difference, plus 10, to the opponent |
| Gin | the opponent's deadwood, plus 20 |
| Box bonus | 20 per hand won |
| Game bonus | 100 for reaching the target first, 200 if the opponent never scored |

The game ends when a player reaches the target score, 100 by default. Only hand
points count towards that target — box and game bonuses are added afterwards, so
the running score during play and the final tally are two different numbers.

## Standard Rummy

**The deal.** Ten cards each head to head, seven for three or four players, six
for five or six. The next card starts the discard pile, the rest is the stock,
and the deal rotates.

**A turn.** Draw one card from the stock or the discard pile; then you may lay
down **one** set or run, and lay off any number of cards onto melds already on
the table, yours or anyone else's; then discard. A card taken from the discard
pile cannot be discarded back on the same turn.

**Going out.** Get rid of every card by melding, laying off, or discarding. The
winner scores the total value of the cards left in the other hands. Shedding the
whole hand in a single turn, having laid nothing down earlier in the hand, is
going rummy and doubles the score.

**When the stock runs out.** The discard pile is turned over — not shuffled — to
form a new stock, so its order carries on.

The game ends when a player reaches the target score, 100 by default.

## House rules

These are deliberate departures, kept because they suit how this app is played.

**Gin with three or more players.** Pagat's Gin Rummy is strictly two-handed
(its three-player form sits the dealer out; four-player is two separate
two-handed games). This engine also seats three or more in one game: hands of
seven, turns passing round the table, the upcard offered to each player in turn.
A knocker collects deadwood from every opponent, the gin bonus is paid once, and
any opponent who undercuts scores as they would head to head.

**Blocked hands in Standard Rummy.** Pagat ends play when the deck has cycled
without progress but gives no rule for scoring it. Here the player holding the
fewest points wins the hand and collects the others' card values; a tie for
lowest washes the hand with no score.

**Agreed draws.** Players may agree to abandon a hand, which washes it with no
score. This has no counterpart in pagat.

**Dealing for the first hand.** Pagat picks the first dealer by drawing cards.
Here the player who created the game deals the first hand; after that the pagat
rotation applies.

**Face-up knocks.** Pagat discards face down when knocking. Everything here is
face up, which changes nothing about the play.
