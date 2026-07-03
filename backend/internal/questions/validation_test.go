package questions

import (
	"testing"
)

func TestValidateAnswer_MCQ(t *testing.T) {
	tests := []struct {
		name          string
		correctAnswer string
		answer        SubmissionAnswer
		expected      bool
	}{
		{"exact match", "O(1)", SubmissionAnswer{TextAnswer: "O(1)"}, true},
		{"trimmed spaces", "O(1)", SubmissionAnswer{TextAnswer: " O(1) "}, true},
		{"case insensitive", "o(1)", SubmissionAnswer{TextAnswer: "O(1)"}, true},
		{"wrong answer", "O(1)", SubmissionAnswer{TextAnswer: "O(N)"}, false},
		{"empty answer", "O(1)", SubmissionAnswer{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAnswer(TypeMCQ, tt.correctAnswer, tt.answer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ValidateAnswer(TypeMCQ) = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestValidateAnswer_Numeric(t *testing.T) {
	floatVal := func(f float64) *float64 { return &f }

	tests := []struct {
		name          string
		correctAnswer string
		answer        SubmissionAnswer
		expected      bool
	}{
		{"exact integer", "10", SubmissionAnswer{NumericAnswer: floatVal(10.0)}, true},
		{"exact string", "10", SubmissionAnswer{TextAnswer: "10"}, true},
		{"floating point decimal", "5.0", SubmissionAnswer{NumericAnswer: floatVal(5.0)}, true},
		{"floating point decimal string", "5.0", SubmissionAnswer{TextAnswer: "5"}, true},
		{"epsilon close enough", "0.3333333333", SubmissionAnswer{NumericAnswer: floatVal(0.33333333331)}, true},
		{"epsilon too far", "0.3333333333", SubmissionAnswer{NumericAnswer: floatVal(0.3333334)}, false},
		{"fallback string exact", "5px", SubmissionAnswer{TextAnswer: "5px"}, true},
		{"fallback string mismatch", "5px", SubmissionAnswer{TextAnswer: "6px"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAnswer(TypeNumericAnswer, tt.correctAnswer, tt.answer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ValidateAnswer(TypeNumericAnswer) = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestValidateAnswer_AlgorithmOrdering(t *testing.T) {
	tests := []struct {
		name          string
		correctAnswer string
		answer        SubmissionAnswer
		expected      bool
	}{
		{"comma separated match", "A,B,C", SubmissionAnswer{TextAnswer: "A,B,C"}, true},
		{"comma separated space mismatch", "A,B,C", SubmissionAnswer{TextAnswer: " A , B , C "}, true},
		{"slice match", "A,B,C", SubmissionAnswer{OrderAnswer: []string{"A", "B", "C"}}, true},
		{"json array correct answer slice match", "[\"A\",\"B\",\"C\"]", SubmissionAnswer{OrderAnswer: []string{"A", "B", "C"}}, true},
		{"json array correct answer comma match", "[\"A\",\"B\",\"C\"]", SubmissionAnswer{TextAnswer: "A,B,C"}, true},
		{"out of order", "A,B,C", SubmissionAnswer{OrderAnswer: []string{"A", "C", "B"}}, false},
		{"shorter input", "A,B,C", SubmissionAnswer{OrderAnswer: []string{"A", "B"}}, false},
		{"empty input", "A,B,C", SubmissionAnswer{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAnswer(TypeAlgorithmOrdering, tt.correctAnswer, tt.answer)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("ValidateAnswer(TypeAlgorithmOrdering) = %v, expected %v", got, tt.expected)
			}
		})
	}
}
