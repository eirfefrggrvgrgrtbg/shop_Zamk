# ZAMK Design Constitution

This document is the stable, project-wide visual constitution for ZAMK. It dictates how the platform should look, feel, and behave, moving beyond generic UI implementations to a deliberate, high-quality product experience.

## A. PRODUCT CHARACTER
ZAMK should feel premium, visually confident, restrained, and purposeful.
- **Shop**: May lean into an editorial, fashion-oriented aesthetic where appropriate.
- **Admin/Seller**: Modern data-product character. Operational, functional, and information-dense when the task requires density, yet sharing the core brand DNA with the Shop.
- **Do NOT look like**: A generic CRM, generic shadcn dashboard, ThemeForest ERP template, Bootstrap admin, or an AI-generated SaaS landing page.

## B. VISUAL HIERARCHY
Whitespace must express hierarchy, not simply make screens empty. Every screen must explicitly establish:
1. Primary user goal
2. Primary information
3. Secondary information
4. Actions
5. Exceptional states
Do not give every element equal visual weight.

## C. TYPOGRAPHY
Define principles over arbitrary one-off font sizes:
- Use clear, sans-serif UI typography for operational products.
- Establish a strong, readable hierarchy.
- Use tabular numerals for money and metrics.
- Maintain deliberate line-heights.
- Avoid tiny gray text everywhere.
- Avoid excessive or unmotivated bolding.
- Editorial typography may be used in the Shop when appropriate.
- Technical/internal terminology must never leak into customer or operator UI.

## D. COLOR
- **ZAMK Violet**: The primary brand/data/action accent. Use it sparingly.
- **Green/Red**: Reserved strictly for semantic state (success/error).
- **Neutral Colors**: Should form the vast majority of the interface.
- **Rules**: Do not create rainbow dashboards. Never encode meaning through color alone.

## E. SURFACES
Use surfaces deliberately:
- **Available treatments**: Border, background shift, grouping, elevation, card.
- **Rule**: A card is NOT the default container.
- Explicitly forbidden: "Card-per-metric" as a default design solution for dashboards.

## F. SPACING / DENSITY
- **Operational pages**: Compact and information-dense where useful to the operator.
- **Commerce/Editorial pages**: More visual breathing room.
- **Rule**: No giant blank areas without a structural or compositional reason.

## G. DATA VISUALIZATION
Charts must only be used when they answer a real business question.
- **Prefer**: Trend, comparison, ranking, composition, and funnel visualizations.
- **Avoid**: Charts purely as decoration.
- **Rule**: Never fabricate a time series or chart when the backend does not provide one.

## H. TABLES
Tables are interactive information systems, not raw HTML grids.
- Establish hierarchy inside the row.
- Define a primary identity column and secondary metadata.
- Ensure strict numeric alignment.
- Implement clear filters, sorting, and empty states.
- Support whole-row navigation when suitable.
- Use action menus only when truly needed.

## I. FORMS
A good form exposes business concepts, not database concepts.
- **Rule**: NEVER ask a human for an internal UUID when a searchable entity picker can be provided.
- **Use**: Searchable selectors/autocompletes, progressive disclosure, logical sections, and contextual help.
- **Avoid**: Endless vertical walls of labels and inputs.

## J. MODALS / DRAWERS / POPOVERS
Choose the interaction model based on task size. Do not automatically put every create/edit flow inside a centered modal.
- **Small decision**: Popover or dialog.
- **Medium structured task**: Drawer or focused modal.
- **Complex workflow**: Dedicated page or workspace.

## K. STATES
Every important surface requires an intentional design for:
- Loading
- Refreshing
- Empty
- Error
- Disabled
- Success
- Partial-data
**Rule**: Never blank the whole application during a data refresh.

## L. MOTION
Motion communicates state and continuity.
- Must be subtle.
- Typical duration: 150–250 ms.
- Avoid purely decorative animation.

## M. RESPONSIVE
Desktop design must not simply collapse accidentally on smaller screens.
- Specify priority preservation (what stays, what hides).
- Use controlled stacking.
- Provide explicit table handling for mobile.
- Adapt mobile interactions intentionally.

## N. ACCESSIBILITY
Design must account for:
- Keyboard navigation and focus rings.
- Adequate contrast.
- Semantic HTML.
- Reduced motion preferences.
- Non-color visual cues.
