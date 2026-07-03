# Domain Model Invariants vs. Business Rules in DSAblitz

This document clarifies the distinction between **Domain Invariants** and **Business Rules** within the context of the **DSAblitz** architecture, detailing where these constraints live and why.

---

## 1. Defining the Boundary

Understanding where constraints are enforced is crucial for maintaining a clean separation of concerns and preventing business logic from bleeding into core data schemas.

### **Domain Invariants (Entity Level)**
* **Definition**: Structural, non-negotiable constraints that must always remain true for a model to exist in a valid state. If a domain invariant is violated, the model is considered structurally corrupted.
* **Scope**: Self-contained. They require no database lookups or external module state to evaluate.
* **Placement**: Inside the domain entity model (e.g., `Question.Validate()` inside `questions/models.go`).
* **Example**: A question with a difficulty of `0` or `99` violates the core grading scale and is a database corruption.

### **Business & Product Rules (Application Level)**
* **Definition**: Policy-driven, behavioral, or feature-specific constraints that govern *how* the application behaves. If a business rule is violated, it represents a validation error (e.g. invalid user input or invalid seed configuration), not structural entity corruption.
* **Scope**: Context-dependent. They may change as product requirements evolve (e.g. game balancing, seeder workflows, or API specifications).
* **Placement**: In service orchestrators, validation helpers, or input adapters (e.g., `questions/seeder.go`, `questions/validation.go`, or `battle/service.go`).
* **Example**: Requiring an MCQ question to have exactly 4 options is a product policy. Technically, a 3-option or 5-option MCQ is structurally valid, but the product chooses to restrict it for match balance.

---

## 2. Constraints Mapping in DSAblitz

Below is a map of where different validation rules are placed in the Questions and Battle modules:

| Rule | Classification | Enforced In | Rationale |
| :--- | :--- | :--- | :--- |
| **UUID Validity** | Domain Invariant | `Question.Validate()` | An entity cannot exist or be queried without a valid, non-nil identifier. |
| **Time Limit Range (10s - 120s)** | Domain Invariant | `Question.Validate()` | Enforces reasonable performance constraints. A question with 0s or 1 hour limits is structurally invalid. |
| **Supported Question Types** | Domain Invariant | `Question.Validate()` | Enforces the system-wide enum constraints on static types. |
| **Title & Prompt Non-Empty** | Domain Invariant | `Question.Validate()` | A question without text or a title is a broken data record. |
| **MCQ Options Count = 4** | Business Rule | `seeder.go` | A product restriction for MCQ templates. In V2, we might allow 3 or 5 options without changing the database schema. |
| **Correct Answer in Options** | Business Rule | `seeder.go` | Ingestion check. A question without its answer in the options list is a content bug. |
| **Ordering Answer Matches Tokens** | Business Rule | `seeder.go` | Content verification check for ordering question steps. |
| **Attempt Limit = 2** | Business Rule | `battle/service.go` | Gameplay mechanic. Can be adjusted to 3 attempts or timed limits in future updates. |
| **Score = 1 per Correct Answer** | Business Rule | `battle/service.go` | Scoring scoring/winner policy. Easily modifiable in V2 for difficulty-weighted scores. |

---

## 3. Structural Implementation Patterns

### **Entity Invariant Verification**
In `backend/internal/questions/models.go`, the entity model contains the validation rule:
```go
func (q *Question) Validate() error {
    if q.ID == uuid.Nil {
        return errors.New("question ID cannot be nil")
    }
    if q.Title == "" || q.Prompt == "" {
        return errors.New("title and prompt cannot be empty")
    }
    if q.Difficulty < 1 || q.Difficulty > 5 {
        return fmt.Errorf("invalid difficulty: %d", q.Difficulty)
    }
    ...
    return nil
}
```

### **Ingestion Business Rules**
In `backend/internal/questions/seeder.go`, we parse inputs, call structural validators, and then check product-specific rules:
```go
// 1. Run structural domain checks
if err := q.Validate(); err != nil {
    return fmt.Errorf("domain invariant error: %w", err)
}

// 2. Run ingestion product checks
if q.QuestionType == TypeMCQ {
    if len(q.Options) != 4 {
        return fmt.Errorf("MCQ must have exactly 4 options, got %d", len(q.Options))
    }
    
    found := false
    for _, opt := range q.Options {
        if cleanCompare(q.CorrectAnswer, opt) {
            found = true
            break
        }
    }
    if !found {
        return fmt.Errorf("MCQ correct answer '%s' not present in options", q.CorrectAnswer)
    }
}
```
