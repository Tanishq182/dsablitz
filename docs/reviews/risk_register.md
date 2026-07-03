# Risk Register: Battle Module

This document catalogs identified risks, likelihoods, impacts, and mitigation plans for the **Battle Module** implementation in **DSAblitz**.

---

## 1. High-Risk Items

### **Concurrent Submissions & Race Conditions**
* **Description**: Multiple players or a single player submitting answers rapidly, causing duplicate scoring or index increments.
* **Impact**: Critical. Players could exploit this to skip questions or gain duplicate points.
* **Mitigation**: Employ row-level pessimistic locking (`SELECT ... FOR UPDATE` on `battle_players` progression row) within the transaction block. This serializes submission updates per player.

### **Transaction Boundaries**
* **Description**: Splitting validation, sequence lookups, submission inserts, and progress updates across multiple independent SQL connections.
* **Impact**: High. Can lead to partial database writes (e.g. progression pointer updates succeeding but submission logs failing), leaving the database in an inconsistent state.
* **Mitigation**: Place all operations within a single, atomic pgx database transaction (`tx`). If any sub-operation fails, the entire transaction rolls back.

---

## 2. Medium-Risk Items

### **Question Progression**
* **Description**: Players advancing past question index 200 (out of bounds) or skipping questions incorrectly.
* **Impact**: Medium. Leads to runtime index errors or broken gameplay loops.
* **Mitigation**: Enforce bounds checks (index < 200) inside `battle.Service` and validate attempts counts (max 2 attempts per question) before updating pointers.

### **Score Consistency**
* **Description**: Mismatch between computed player stats (correct/incorrect counts) and logged entries in the submissions table.
* **Impact**: Medium. Affects match results, standings, and historical stats.
* **Mitigation**: Calculate scoring values inside the same locked database transaction that updates the player progress counters.

---

## 3. Low-Risk Items

### **DTO Mapping**
* **Description**: Failure to map database entities to internal Go DTOs or WebSocket payloads.
* **Impact**: Low. Can be caught early through unit testing.
* **Mitigation**: Write unit tests for JSON marshaling and payload conversions.

### **Logging**
* **Description**: Insufficient debugging logs for system errors during active matches.
* **Impact**: Low. Slows down bug investigations but does not corrupt data.
* **Mitigation**: Integrate clean structured logging using standard log packages throughout the service.
