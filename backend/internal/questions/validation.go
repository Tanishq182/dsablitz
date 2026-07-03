package questions

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const epsilon = 1e-9

// SubmissionAnswer represents the strongly-typed user answer structure.
type SubmissionAnswer struct {
	TextAnswer    string   `json:"text_answer,omitempty"`
	NumericAnswer *float64 `json:"numeric_answer,omitempty"`
	OrderAnswer   []string `json:"order_answer,omitempty"`
}

// ValidateAnswer compares the correct answer with the user's submitted answer DTO
// based on the question type. It returns true if they match, false otherwise.
func ValidateAnswer(qType QuestionType, correctAnswer string, answer SubmissionAnswer) (bool, error) {
	switch qType {
	case TypeMCQ, TypeComplexityPrediction:
		return cleanCompare(correctAnswer, answer.TextAnswer), nil

	case TypeNumericAnswer:
		correctFloat, okCorrect := parseFloat64(correctAnswer)
		if !okCorrect {
			// If correct answer in DB isn't a valid float, fallback to string compare
			return cleanCompare(correctAnswer, answer.TextAnswer), nil
		}

		if answer.NumericAnswer != nil {
			return math.Abs(correctFloat-*answer.NumericAnswer) < epsilon, nil
		}

		// Fallback to parsing text answer
		userFloat, okUser := parseFloat64(answer.TextAnswer)
		if okUser {
			return math.Abs(correctFloat-userFloat) < epsilon, nil
		}
		return cleanCompare(correctAnswer, answer.TextAnswer), nil

	case TypeAlgorithmOrdering:
		correctSlice := parseStringSlice(correctAnswer)
		userSlice := answer.OrderAnswer

		// Fallback: If OrderAnswer is empty but TextAnswer is populated, try parsing it
		if len(userSlice) == 0 && answer.TextAnswer != "" {
			userSlice = parseStringSlice(answer.TextAnswer)
		}

		if len(correctSlice) != len(userSlice) {
			return false, nil
		}

		for i := range correctSlice {
			if !cleanCompare(correctSlice[i], userSlice[i]) {
				return false, nil
			}
		}
		return true, nil

	default:
		return false, fmt.Errorf("unsupported question type: %s", qType)
	}
}

func cleanCompare(a, b string) bool {
	return strings.TrimSpace(strings.ToLower(a)) == strings.TrimSpace(strings.ToLower(b))
}

func parseFloat64(val string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
	if err == nil {
		return f, true
	}
	return 0, false
}

// parseStringSlice parses a comma-separated string or a JSON array of strings
func parseStringSlice(str string) []string {
	str = strings.TrimSpace(str)
	if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {
		var slice []string
		if err := json.Unmarshal([]byte(str), &slice); err == nil {
			return slice
		}
	}

	parts := strings.Split(str, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
