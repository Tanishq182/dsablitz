package questions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// SeedQuestions parses a JSON file containing the question bank and seeds it into the database.
// It executes domain validations and product checks before committing.
func SeedQuestions(ctx context.Context, repo *Repository, filePath string) (int, error) {
	if filePath == "" {
		return 0, errors.New("seed file path is required")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("read seed file: %w", err)
	}

	var questionsList []Question
	if err := json.Unmarshal(data, &questionsList); err != nil {
		return 0, fmt.Errorf("unmarshal seed questions: %w", err)
	}

	// Maps to detect file-level duplicates
	seenIDs := make(map[uuid.UUID]bool)
	seenTitles := make(map[string]bool)

	// Validate all questions first (all-or-nothing check)
	for i, q := range questionsList {
		// 1. Domain Invariant Validations
		if err := q.Validate(); err != nil {
			return 0, fmt.Errorf("question at index %d ('%s') failed domain invariants: %w", i, q.Title, err)
		}

		// 2. Duplicate Detection
		if seenIDs[q.ID] {
			return 0, fmt.Errorf("duplicate question ID found in seed file: %s", q.ID)
		}
		seenIDs[q.ID] = true

		trimmedTitle := strings.TrimSpace(strings.ToLower(q.Title))
		if seenTitles[trimmedTitle] {
			return 0, fmt.Errorf("duplicate question title found in seed file: %s", q.Title)
		}
		seenTitles[trimmedTitle] = true

		// 3. Product & Business Validation Rules
		switch q.QuestionType {
		case TypeMCQ:
			if len(q.Options) != 4 {
				return 0, fmt.Errorf("MCQ question '%s' must have exactly 4 options, got %d", q.Title, len(q.Options))
			}
			// Correct answer must exist in options
			found := false
			for _, opt := range q.Options {
				if cleanCompare(q.CorrectAnswer, opt) {
					found = true
					break
				}
			}
			if !found {
				return 0, fmt.Errorf("MCQ question '%s' correct answer '%s' is not present in options list", q.Title, q.CorrectAnswer)
			}

		case TypeComplexityPrediction:
			if len(q.Options) > 0 {
				return 0, fmt.Errorf("complexity prediction question '%s' must not have options", q.Title)
			}
			// Verify it matches basic big-O syntax
			ans := strings.TrimSpace(q.CorrectAnswer)
			if !strings.HasPrefix(ans, "O(") || !strings.HasSuffix(ans, ")") {
				return 0, fmt.Errorf("complexity prediction question '%s' answer '%s' must follow O(...) format", q.Title, q.CorrectAnswer)
			}

		case TypeNumericAnswer:
			if len(q.Options) > 0 {
				return 0, fmt.Errorf("numeric answer question '%s' must not have options", q.Title)
			}
			// Verify answer parses as integer
			if _, err := strconv.Atoi(strings.TrimSpace(q.CorrectAnswer)); err != nil {
				return 0, fmt.Errorf("numeric answer question '%s' answer '%s' must be an integer: %w", q.Title, q.CorrectAnswer, err)
			}

		case TypeAlgorithmOrdering:
			if len(q.Options) < 2 {
				return 0, fmt.Errorf("ordering question '%s' must have at least 2 options", q.Title)
			}
			// Verify correct answer contains matching number of tokens
			tokens := parseStringSlice(q.CorrectAnswer)
			if len(tokens) != len(q.Options) {
				return 0, fmt.Errorf("ordering question '%s' correct answer token count (%d) must match options count (%d)", q.Title, len(tokens), len(q.Options))
			}

		default:
			return 0, fmt.Errorf("question '%s' has unsupported type: %s", q.Title, q.QuestionType)
		}
	}

	// Perform database seeding
	seededCount := 0
	for _, q := range questionsList {
		err = repo.InsertOrUpdateQuestion(ctx, q)
		if err != nil {
			return seededCount, fmt.Errorf("seed database question '%s': %w", q.Title, err)
		}
		seededCount++
	}

	return seededCount, nil
}
