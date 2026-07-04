package ratings

import (
	"testing"

	"github.com/google/uuid"
)

func TestEloEngine_Calculate(t *testing.T) {
	engine := NewEloEngine(32)
	p1ID := uuid.New()
	p2ID := uuid.New()

	tests := []struct {
		name         string
		rating1      int
		rating2      int
		outcome      MatchOutcome
		expectedDel1 int
		expectedDel2 int
	}{
		{
			name:         "equal ratings - player 1 wins",
			rating1:      1000,
			rating2:      1000,
			outcome:      Player1Win,
			expectedDel1: 16,
			expectedDel2: -16,
		},
		{
			name:         "equal ratings - player 2 wins",
			rating1:      1000,
			rating2:      1000,
			outcome:      Player2Win,
			expectedDel1: -16,
			expectedDel2: 16,
		},
		{
			name:         "equal ratings - draw",
			rating1:      1000,
			rating2:      1000,
			outcome:      Draw,
			expectedDel1: 0,
			expectedDel2: 0,
		},
		{
			name:         "upset victory - lower rated player 1 wins",
			rating1:      1000,
			rating2:      1400,
			outcome:      Player1Win,
			expectedDel1: 29,
			expectedDel2: -29,
		},
		{
			name:         "upset victory - lower rated player 2 wins",
			rating1:      1400,
			rating2:      1000,
			outcome:      Player2Win,
			expectedDel1: -29,
			expectedDel2: 29,
		},
		{
			name:         "standard victory - higher rated player 1 wins",
			rating1:      1400,
			rating2:      1000,
			outcome:      Player1Win,
			expectedDel1: 3,
			expectedDel2: -3,
		},
		{
			name:         "draw with rating gap",
			rating1:      1400,
			rating2:      1000,
			outcome:      Draw,
			expectedDel1: -13,
			expectedDel2: 13,
		},
		{
			name:         "extreme rating gap - player 1 wins",
			rating1:      800,
			rating2:      2400,
			outcome:      Player1Win,
			expectedDel1: 32,
			expectedDel2: -32,
		},
		{
			name:         "extreme rating gap - player 1 loses",
			rating1:      800,
			rating2:      2400,
			outcome:      Player2Win,
			expectedDel1: 0,
			expectedDel2: 0,
		},
		{
			name:         "invalid outcome defaults to draw",
			rating1:      1000,
			rating2:      1000,
			outcome:      MatchOutcome(999),
			expectedDel1: 0,
			expectedDel2: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p1 := PlayerRating{UserID: p1ID, Rating: Rating{Value: tt.rating1}}
			p2 := PlayerRating{UserID: p2ID, Rating: Rating{Value: tt.rating2}}

			up1, up2 := engine.Calculate(p1, p2, tt.outcome)

			if up1.Delta != tt.expectedDel1 {
				t.Errorf("Calculate() got delta1 = %v, want %v", up1.Delta, tt.expectedDel1)
			}
			if up2.Delta != tt.expectedDel2 {
				t.Errorf("Calculate() got delta2 = %v, want %v", up2.Delta, tt.expectedDel2)
			}
		})
	}
}

func TestEloEngine_RepeatedMatches(t *testing.T) {
	engine := NewEloEngine(32)
	p1ID := uuid.New()
	p2ID := uuid.New()

	p1 := PlayerRating{UserID: p1ID, Rating: Rating{Value: 1000}}
	p2 := PlayerRating{UserID: p2ID, Rating: Rating{Value: 1000}}

	// Player A wins 5 times in a row
	for i := 0; i < 5; i++ {
		up1, up2 := engine.Calculate(p1, p2, Player1Win)
		p1.Rating = up1.After
		p2.Rating = up2.After
	}

	if p1.Rating.Value != 1067 || p2.Rating.Value != 933 {
		t.Errorf("Repeated matches got r1 = %d, r2 = %d; want r1 = 1067, r2 = 933", p1.Rating.Value, p2.Rating.Value)
	}
}

func TestEloEngine_FloatingPointDriftAndZeroFloor(t *testing.T) {
	engine := NewEloEngine(32)
	p1ID := uuid.New()
	p2ID := uuid.New()

	p1 := PlayerRating{UserID: p1ID, Rating: Rating{Value: 1653}}
	p2 := PlayerRating{UserID: p2ID, Rating: Rating{Value: 1121}}

	up1, up2 := engine.Calculate(p1, p2, Draw)

	if up1.Delta != -15 || up2.Delta != 15 {
		t.Errorf("Drift test got d1=%d, d2=%d; want d1=-15, d2=15", up1.Delta, up2.Delta)
	}
	if up1.Delta+up2.Delta != 0 {
		t.Errorf("Drift test sum of deltas: %d; want 0 (zero-sum)", up1.Delta+up2.Delta)
	}

	// Verify floor at zero
	p1_zero := PlayerRating{UserID: p1ID, Rating: Rating{Value: 10}}
	p2_zero := PlayerRating{UserID: p2ID, Rating: Rating{Value: 10}}
	up1_zero, _ := engine.Calculate(p1_zero, p2_zero, Player2Win)

	if up1_zero.After.Value != 0 {
		t.Errorf("expected floor capped at 0, got %d", up1_zero.After.Value)
	}
	if up1_zero.Delta != -10 {
		t.Errorf("expected delta to reflect floor change of -10, got %d", up1_zero.Delta)
	}
}
