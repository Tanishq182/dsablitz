package ratings

import (
	"math"
)

// EloEngine implements the RatingEngine interface using the standard Elo formula.
type EloEngine struct {
	kFactor int
}

// NewEloEngine creates a new EloEngine with the given K-factor.
func NewEloEngine(kFactor int) *EloEngine {
	return &EloEngine{kFactor: kFactor}
}

// Calculate computes rating updates for Player A and Player B.
func (e *EloEngine) Calculate(playerA, playerB PlayerRating, outcome MatchOutcome) (RatingUpdate, RatingUpdate) {
	rA := float64(playerA.Rating.Value)
	rB := float64(playerB.Rating.Value)

	expA := 1.0 / (1.0 + math.Pow(10.0, (rB-rA)/400.0))
	expB := 1.0 / (1.0 + math.Pow(10.0, (rA-rB)/400.0))

	var sA, sB float64
	switch outcome {
	case Player1Win:
		sA, sB = 1.0, 0.0
	case Player2Win:
		sA, sB = 0.0, 1.0
	case Draw:
		sA, sB = 0.5, 0.5
	default:
		sA, sB = 0.5, 0.5
	}

	deltaA := int(math.Round(float64(e.kFactor) * (sA - expA)))
	deltaB := int(math.Round(float64(e.kFactor) * (sB - expB)))

	newRA := playerA.Rating.Value + deltaA
	newRB := playerB.Rating.Value + deltaB

	// Enforce floor rating of 0
	if newRA < 0 {
		newRA = 0
	}
	if newRB < 0 {
		newRB = 0
	}

	updateA := RatingUpdate{
		Before: playerA.Rating,
		After:  Rating{Value: newRA},
		Delta:  newRA - playerA.Rating.Value,
	}

	updateB := RatingUpdate{
		Before: playerB.Rating,
		After:  Rating{Value: newRB},
		Delta:  newRB - playerB.Rating.Value,
	}

	return updateA, updateB
}
