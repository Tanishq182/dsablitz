# Documentation Quality & Compliance Checklist

This document answers the primary question: **What criteria must all DSAblitz documentation meet to remain production-ready and interview-aligned?**

---

## 1. Reality-First Standard
* **Constraint**: Document only features that are implemented in the codebase.
* **Rule**: Future roadmaps, designs, or thoughts must be clearly marked under a `## Planned Work (V2)` header or placed inside the `docs/roadmap/` directory.

## 2. Alternatives & Tradeoffs Design Philosophy
Every design or architectural document must contain:
* **Why Not?**: Explicitly detail what alternatives were considered and rejected (e.g. why not microservices, why not Redis, why not WebSockets, why not standard library http routers).
* **Tradeoffs**: Clearly define:
  * **Pros**: Advantages of the selected design.
  * **Cons**: Disadvantages and runtime performance overheads.
  * **Limitations**: Scenarios where the design fails or requires scale modifications.
  * **Future Improvements**: Short-term adjustments.

## 3. Strict Code-Level References
* **Rule**: Avoid generic descriptions. All references to components must link directly to the implementation files and list the exact code symbols:
  * **Incorrect**: "The auth middleware verifies the token."
  * **Correct**: "The auth middleware [middleware.go](file:///home/tanishq/dsablitz/backend/internal/auth/middleware.go#L15) parses incoming requests, extract claims via `ValidateToken()`, and maps user context parameters."

## 4. End-of-File Structure
* **Rule**: Every document must conclude with these three sections:
  1. **Key Takeaways**: High-level summaries.
  2. **Common Interview Questions**: 1-2 typical interviewer questions about the document topic.
  3. **Related Documents**: Relative clickable file links ensuring zero orphan markdown files.

## 5. Visuals & Diagrams
* **Rule**: Use Mermaid sequence diagrams, state machines, module interactions, request flows, and package dependencies wherever logical flow is described.

---

## Key Takeaways
1. A clean codebase is only as good as its documentation.
2. Aligning documents with these templates prepares developers directly for engineering reviews and technical interviews.

## Common Interview Questions
* **How do you ensure documentation does not drift from implementation?**
  * *Answer*: By enforcing a quality checklist and requiring phase sign-offs that verify code changes are immediately mirrored in design docs.

## Related Documents
* For system context, see [PROJECT_CONTEXT.md](file:///home/tanishq/dsablitz/docs/PROJECT_CONTEXT.md).
* For overall layout, see [overall_architecture.md](file:///home/tanishq/dsablitz/docs/architecture/overall_architecture.md).
