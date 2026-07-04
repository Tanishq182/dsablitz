# Risk Register: Ratings Module (Phase 8A)

This document catalogs identified risks, likelihoods, impacts, and mitigation plans for the **Ratings Module** implementation in **DSAblitz**.

---

## 1. High-Risk Items

### **Concurrent Rating Updates & Deadlocks**
* **Description**: If two battles involving shared players complete at the same time, concurrent transactions could attempt to lock user statistics in reverse order (User A then User B, vs. User B then User A).
* **Impact**: High. Induces database deadlock aborts, rolling back battle completions.
* **Mitigation**: Enforce a strict, deterministic lock order. We sort player UUIDs alphabetically/lexicographically and query/lock them sequentially:
  ```go
  if p1ID.String() < p2ID.String() {
      // Lock p1 first, then p2
  } else {
      // Lock p2 first, then p1
  }
  ```

### **Double Rating Adjustments**
* **Description**: A battle completion function being executed twice due to network retries, causing the rating change to be applied twice.
* **Impact**: High. Distorts matchmaking ratings (MMR) and leaderboard standing.
* **Mitigation**: Introduce a database-level composite unique constraint on `rating_history(battle_id, user_id)` (to be implemented in Phase 8B) which automatically fails any duplicate rating logs.

---

## 2. Medium-Risk Items

### **Rating Decoupling Integrity**
* **Description**: The Battle module directly accessing the database mapping of the ratings module, causing coupling.
* **Impact**: Medium. Restricts our ability to swap Elo with Glicko-2 in V2.
* **Mitigation**: Use dependency inversion. The Battle service interacts only with the `RatingCoordinator` interface, remaining blind to the underlying rating formulas or history repository.

### **Negative Ratings**
* **Description**: A long losing streak dropping a player's rating below zero.
* **Impact**: Medium. Breaking constraints or displaying negative numbers on profiles.
* **Mitigation**: Enforce a strict rating floor of `0` in `Service.ApplyRatingUpdatesTx` before saving ratings to `user_stats`.

---

## 3. Low-Risk Items

### **Floating-Point Precision Drifts**
* **Description**: Small rounding errors in expected score floats accumulating over hundreds of matches, causing rating values to drift.
* **Impact**: Low. Minor rating variation, but does not affect database integrity.
* **Mitigation**: Use Go's `math.Round` on float calculations before casting to integer, and verify zero-sum behavior (net rating change is balanced) in tests.
