# Customer Profile and Name Field Handling

## Overview
This document describes the design decisions and implementation details for customer name fields. In the transition from a single `name` column to structured `first_name`, `last_name`, `middle_name` fields, backward compatibility and strict input validation have been prioritized.

## Migration (000026)
Added `first_name`, `last_name`, `middle_name` columns to the `users` table as nullable strings. The existing `name` column is retained as a cached, full-name representation to maintain backward compatibility with components (like Seller Order Management and fulfillment features) that do not need fine-grained name details.

## Validation Policy
Customer inputs for names are strictly validated:
- **Length**: `FirstName`, `LastName`, and `MiddleName` are capped at 80 characters.
- **Character Set**: Allowed characters include Latin (`a-zA-Z`), Cyrillic (`а-яА-ЯёЁ`), spaces, hyphens (`-`), and apostrophes (`'`). Numbers and other special characters are disallowed.
- **Junk Prevention**: Inputs containing exclusively punctuation (e.g. `---` or `...`) are rejected.
- **Required Fields**: `FirstName` and `LastName` are mandatory. `MiddleName` is optional.

## Registration Flow
When a customer registers:
1. `ValidateNameFields()` validates the `firstName`, `lastName`, and `middleName`.
2. A single `FullName` string is dynamically composed (`LastName + FirstName + MiddleName`).
3. This `FullName` is mapped to the legacy `name` column.
4. The structured fields are also saved to their respective new columns.

## Profile Updates
When a customer updates their profile:
1. They must provide the 3 name fields via the `/users/me/profile` endpoint.
2. The endpoint updates both the legacy `name` column and the individual fields.

## Privacy Rules
To protect customer PII:
- **Seller Visibility**: Sellers only receive the unified `fullName` string when necessary (e.g. inside `orders.SellerOrder` if specifically required, though currently orders may omit it depending on the exact fulfillment phase). Sellers NEVER receive the raw `firstName`/`lastName` splits for customers directly, except via fulfillment address labels which are explicitly handled.
- **Fulfillment / Notifications**: Fulfillment DTOs and notification channels use the `SellerName` or `FullName` constructs without leaking the separated name struct.
