# Visual Acceptance

This document separates the concepts of technical correctness, mockup visual direction, and live implementation visual quality.

**Explicit Rule: TECHNICAL PASS ≠ VISUAL PASS.**
A page may be technically perfect (passes tests, compiles, fetches data) and still be rejected visually. Visual acceptance requires real browser evidence and Product Owner review.

**Explicit Rule: MOCKUP ACCEPTANCE ≠ LIVE IMPLEMENTATION ACCEPTANCE.**
Approving an external HTML/CSS mockup (Gate A) selects visual direction and architecture. It does NOT constitute acceptance of the live production implementation (Gate B / Final), which must be verified against real backend data and edge cases in the browser.

---

## 1. Technical Acceptance Gate
This gate is handled by standard CI and automated verification loops.
- All unit and integration tests pass cleanly on `zamk_test`.
- Build compiles with 0 errors and 0 warnings.
- Type safety and linting are strictly maintained across frontend and backend.
- API requests, responses, and serialization are correct.
- RBAC permissions are accurately enforced on endpoints and client views.
- Database migrations execute cleanly with deterministic rollbacks.
- Baseline accessibility (ARIA roles, keyboard focus, valid semantic HTML) is met.

*Technical acceptance is necessary but NEVER sufficient for shipping UI.*

---

## 2. Mockup Visual Acceptance (Gate A — Direction & Composition)
This gate evaluates visual exploration artifacts (HTML/CSS prototypes outside the repository).

### Checklist for Mockup Review:
1. **Composition & Hierarchy**:
   - Does the eye immediately go to the primary operational element?
   - Is secondary information appropriately de-emphasized without becoming unreadable?
   - Does whitespace establish structural hierarchy rather than empty gaps?
2. **Information Density (10–15s Viewport Test)**:
   - Can an operator determine primary metric, trend, cause, and required action within 10–15 seconds on the first viewport?
3. **Multi-State Integrity**:
   - Are Zero and Sparse states intentionally designed (not just empty table frames or broken charts)?
4. **Product Truth Alignment**:
   - Are all displayed metrics backed by real domain concepts (no invented ROAS/CAC/LTV without spend data)?
   - Are future or unsupported controls explicitly marked (`REQUIRES FUTURE ENDPOINT`)?
5. **Anti-AI Pattern Audit**:
   - Free of card soup, pill abuse, generic SaaS heroes, unmotivated gradients, and excessive rounded borders.

---

## 3. Live Implementation Visual Acceptance (Gate B / Final — Real-Data Browser QA)
This gate can ONLY be cleared by the Product Owner reviewing live browser evidence with production-shaped data.

### Live Data Stress-Test Checklist:
When submitting a live implementation milestone, the agent must verify and provide evidence across these real-data conditions:

1. **Zero State (Cold Start)**:
   - First-time user experience with 0 records.
   - Clear explanatory copy and unambiguous primary call-to-action.
   - No raw empty tables, broken axes, or `NaN`/`undefined` metrics.

2. **Sparse State (Initial Activity)**:
   - 1 or 2 records, single-digit operational numbers (e.g., 1 session, 8 orders, 58 398 ₽).
   - Charts render gracefully without geometric distortion or division-by-zero errors.
   - Summaries and cards maintain proportional visual balance.

3. **Realistic / Normal State**:
   - Typical data volume, varied status badges, realistic date ranges.
   - Tabular numbers strictly right-aligned; text left-aligned.
   - Enums and internal statuses rendered as human-readable labels.

4. **Dense / High-Volume State**:
   - Many rows, large monetary sums, tight tabular alignment.
   - Pagination, scrolling, and sticky headers behave predictably without horizontal blowout.

5. **Anomalous / Boundary Conditions**:
   - Counter-intuitive operational data (e.g., revenue with 0 attributed sessions, 100% discount, 0 ₽ balance).
   - Extreme values (9-digit numbers, long entity names, multi-line titles).
   - Text wrapping and truncation without breaking layout geometry.

6. **Async & Failure States**:
   - Loading skeletons or spinners preserve layout stability (no layout shift / CLS).
   - Network errors, 403 Forbidden, 404 Not Found, and 500 Server Error display actionable retry or contact guidance.

7. **RBAC & Permission Boundaries**:
   - Unauthorized actions are hidden or disabled with clear tooltips.
   - Seller view never leaks Admin platform-wide metrics or cross-seller data.

---

## 4. Acceptance Evidence Protocol
To request Product Owner visual acceptance:
1. Provide exact URLs / routes to inspect.
2. Provide specific test accounts or roles (Admin, Seller, etc.).
3. Document which data states were tested and link any visual screenshots or recordings.
4. Never mark visual acceptance as PASS without explicit Product Owner confirmation.
