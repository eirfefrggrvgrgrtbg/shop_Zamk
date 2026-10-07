---
name: zamk-frontend-design
description: Permanent ZAMK frontend design layer workflow. Mandatory for all UI changes to avoid generic AI-generated interfaces and ensure adherence to the ZAMK Design Constitution.
---

# ZAMK Frontend Design Skill

This skill enforces the product-design workflow for Antigravity. It ensures ZAMK avoids generic AI-generated admin patterns and maintains a premium, deliberate, and high-quality visual language.

## CORE PRINCIPLES

### 1. DESIGN-ONLY = REPOSITORY READ-ONLY
When a task is in **DESIGN-ONLY** or **VISUAL EXPLORATION** phase:
**The repository product files are strictly READ-ONLY.**

- **The agent MAY**:
  - Read repository source and configuration
  - Inspect components, hooks, styles, and routing
  - Inspect backend APIs, DTOs, schemas, and data contracts
  - Inspect tests and test suites
  - Run read-only audit commands
  - Create temporary visual artifacts (HTML, CSS, mockups) **OUTSIDE the repository** (in Antigravity artifact directory or `/tmp`)
- **The agent MUST NOT**:
  - Write or edit production source files
  - Write or edit frontend or backend tests
  - Add or modify API client types or schemas
  - Create or edit database migrations
  - Create production components or route definitions
  - Modify styles or CSS in the repository
  - Create helper scripts, temp files, or wrappers inside the repository
  - "Prepare implementation" or "make the mockup real" before explicit authorization
  - Continue into coding after producing visual artifacts

> [!CAUTION]
> Any repository write during a DESIGN-ONLY phase is a **DESIGN-GATE VIOLATION**. If attempted, the agent must **STOP immediately**, report the violation, and refuse to continue implementation.

### 2. TEMPORARY ARTIFACT LOCATION
For all DESIGN-ONLY / prototype work:
- Prototypes and visual mockups must live **ONLY outside the repository**:
  - Antigravity brain artifact directory (`<appDataDir>/brain/<conversation-id>/...`)
  - `/tmp/...`
- **NEVER** place HTML mockups, static screenshots, prototype CSS, mock JSON files, or temporary React components inside the repository tree.

### 3. PRODUCT TRUTH OVERRIDES VISUAL MOCKUP
A design mockup must NEVER invent business truth or feature capabilities just because they look appealing.
- Operational metrics require genuine data backing (e.g., ROAS requires actual spend truth; CAC requires acquisition cost truth; LTV requires canonical lifetime value semantics).
- Domain rules cannot be altered for visual convenience (e.g., internal campaign purpose classification cannot be exposed as an editable user dropdown without domain architecture changes).
- Unsupported features or future controls must be explicitly marked:
  - `REQUIRES PRODUCT/BACKEND DECISION` or `REQUIRES FUTURE ENDPOINT`
- Never silently convert design fiction into production implementation.

### 4. OVERLAY CONTEXT RULE (DRAWER VS DEDICATED PAGE)
- A **drawer / side panel** is appropriate ONLY when the underlying screen context remains meaningful and useful while the overlay is open.
- If the workflow:
  - Heavily darkens/masks the background
  - Obscures nearly all underlying context
  - Requires large form or configuration space
  - Represents a multi-step or core business creation flow
- **PREFER**: A dedicated page / workspace (e.g., `/marketing/campaigns/new`) over a drawer.
- Never choose a modal or drawer merely because it feels faster or easier to scaffold.

### 5. REAL-DATA STRESS TEST
For all analytics and data-heavy surfaces:
Before proposing or approving a visual composition, stress-test it against 5 distinct data realities:
1. **ZERO**: Clean zero-state with intentional onboarding/guidance (no raw table headers with empty rows).
2. **SPARSE**: Single-digit entries (e.g., 1 session, 8 orders, 58 398 ₽) without breaking chart geometry or rendering NaN/undefined.
3. **NORMAL**: Realistic average distribution.
4. **DENSE**: High item volume and tight tabular numbers.
5. **ANOMALOUS**: Counter-intuitive data (e.g., orders without attribution, high revenue with low conversion) with deterministic explanations.
A layout that works only with ideal demo numbers is an AI failure mode (**IDEAL-DATA THEATER**). The agent must explicitly report how each state behaves.

### 6. INFORMATION DENSITY CHECK (10–15 SECOND VIEWPORT TEST)
For operational Admin and Seller dashboards:
- *What operational decisions can the user make from the first viewport within 10–15 seconds?*
- The first viewport must clearly communicate:
  1. **Primary outcome**: Core operational metric/status.
  2. **Trend**: Directional change over time.
  3. **Cause / Source**: Where the activity or volume originated.
  4. **Operational action**: Immediate next step or primary action.
  5. **Anomaly / Attention**: Items requiring intervention.
- Do not solve low density by adding card soup or useless drop shadows.

---

## TASK CLASSES & WORKFLOWS

### CLASS S — SMALL
**Scope**: Minor visual adjustments (icon changes, copy updates, spacing corrections, isolated component states).
**Workflow**:
1. Inspect current design language.
2. Implement code.
3. Run automated tests and build.
4. Request visual review.
*(No concept gate required).*

### CLASS M — MEDIUM
**Scope**: Standard component or section work (new panel, significant form, table redesign, page subsection).
**Workflow**:
1. UX audit of target area.
2. Define information hierarchy.
3. Create lightweight composition/wireframe.
4. Implement code.
5. Provide visual comparison for review.

### CLASS L — LARGE / REDESIGN
**Scope**: Major features, core flows, or full page redesigns (e.g., Marketing, Product Studio, Dispatch, Seller dashboard, Navigation).

#### MANDATORY PHASE LOCK FOR CLASS L
At the start of every Class L task, the agent must declare the active phase:
- `PHASE: RESEARCH`
- `PHASE: DESIGN-ONLY`
- `PHASE: DESIGN SPEC`
- `PHASE: IMPLEMENTATION`
- `PHASE: VISUAL QA`

The agent **CANNOT silently advance** to the next phase. Phase transition requires explicit user instructions authorizing that transition.

#### TWO DISTINCT GATES FOR CLASS L
1. **GATE A — VISUAL DIRECTION APPROVAL**:
   - Product Owner selects/approves a visual direction from concepts.
   - Permits creation of final design spec/mockup.
   - **DOES NOT authorize production coding** unless the current prompt explicitly permits implementation.
2. **GATE B — IMPLEMENTATION AUTHORIZATION**:
   - Production implementation begins ONLY when the latest user request explicitly instructs the agent to implement the approved design.
   - Never infer permission from "looks good", design artifact completion, or green tests.

#### CLASS L EXECUTION STEPS:
1. **PRODUCT TRUTH**: Review domain rules, data contracts, permissions, states, and constraints.
2. **UX AUDIT**: Identify exactly what is wrong with the current UX (attention, density, leakage, hierarchy).
3. **REFERENCE RESEARCH**: Study modern references (`reference-playbook.md`). Extract principles, not screenshots.
4. **INFORMATION ARCHITECTURE**: Define primary, secondary, actionable, and collapsible elements.
5. **VISUAL CONCEPTS**: Produce 2–3 meaningfully distinct concepts (or one final spec if direction is locked) as external HTML artifacts.
6. **REAL-DATA AUDIT**: Verify mockups against Zero, Sparse, Normal, Dense, and Anomalous data states.
7. **OWNER VISUAL DIRECTION GATE (GATE A)**:
   - Present artifacts to Product Owner.
   - **MANDATORY HARD STOP**:
     ```
     PHASE: DESIGN-ONLY COMPLETE
     PRODUCTION WRITES: NONE
     WAITING FOR OWNER DIRECTION
     ```
   - Do NOT edit repository files. Do NOT start coding while waiting.
8. **DESIGN SPEC**: After Gate A approval, define exact geometry, typography, spacing, states, and breakpoints.
9. **IMPLEMENTATION (GATE B)**: Once explicitly authorized by owner, write production code strictly following the approved concept.
10. **LIVE IMPLEMENTATION COMPARISON**: Compare real browser output against the approved design with live production-shaped data (see `visual-acceptance.md`).
11. **OWNER FINAL VISUAL ACCEPTANCE**: Technical tests do NOT equal visual acceptance. Only the owner can grant the final visual PASS.

---

## DESIGN REVIEW QUESTIONS
Before submitting any frontend work, verify:
- What should the eye notice first?
- Is every visible element earning its visual weight?
- Is whitespace creating hierarchy or merely emptiness?
- Could this interface be mistaken for a generic AI dashboard?
- Are we solving the user's operational job or merely dumping database fields?
- Are internal identifiers leaking into UI?
- Are we using cards because they are needed or because they are easy?
- Does this feel like ZAMK (restrained, serif headings, strict tabular numbers, purposeful violet)?
- Is the information density appropriate for the surface?

## SUPPORTING DOCUMENTS
- `DESIGN.md` (Repository root) — The ZAMK visual constitution.
- `anti-ai-patterns.md` — Forbidden generic AI UI patterns and failure modes.
- `reference-playbook.md` — Guidelines for researching UX principles.
- `visual-acceptance.md` — Technical pass vs. visual pass distinction.
