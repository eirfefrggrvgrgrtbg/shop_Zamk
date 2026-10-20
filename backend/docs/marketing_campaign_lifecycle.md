# Advertising campaign lifecycle (MARKETING.M1.3)

Applies to `marketing_campaigns.purpose = 'advertising'`. Promotion campaigns (seller promos,
co-funding) keep their own flow and are not changed.

## States and transitions

| DB status   | Meaning (PO name)        | Accepts tracking traffic | Editable |
|-------------|--------------------------|--------------------------|----------|
| `draft`     | DRAFT                    | no                       | yes (may be incomplete) |
| `approved`  | APPROVED, ready to start | no (see legacy rule)     | yes |
| `active`    | ACTIVE                   | yes, inside the period   | yes |
| `paused`    | PAUSED                   | no                       | yes |
| `ended`     | COMPLETED (terminal)     | no                       | no |
| `cancelled` | CANCELLED (terminal)     | no                       | no |
| `rejected`, `submitted`, `counter_offered` | legacy | no | rejected: no; others: cancel only |

`ended` is kept as the stored value of COMPLETED so existing rows, analytics and API clients stay valid.

```
draft -> approved -> active <-> paused
active | paused -> ended
draft | approved | active | paused (| submitted | counter_offered) -> cancelled
```

Enforced in `PATCH /api/admin/marketing/campaigns/{id}` under a row lock (`SELECT ... FOR UPDATE`):

- `409 invalid_status_transition` — transition not in the list (incl. `active -> approved`);
- `409 campaign_closed` — any change of a completed/cancelled/rejected campaign (an idempotent repeat
  of its own status is accepted);
- `422 campaign_not_ready` — leaving draft requires `title`, `campaignChannel`, `campaignType`, `startsAt`;
- `400 invalid_parameter` — `endsAt` not after `startsAt`.

New campaigns are created as `draft` (`POST` returns `status: "draft"`). Tracking links can be created
for draft/approved/active/paused campaigns and are refused (`409 campaign_closed`) for closed ones.

## Traffic rule

`CheckCampaignAcceptsTraffic` is evaluated on every redirect (`/r/{token}`) and every session token
resolution (attribution): status `active`, and the half-open period `starts_at <= now < ends_at`.
A pause blocks new redirects and new campaign sessions; links, sessions, order attributions and
analytics are kept. Orders placed during a pause by visitors whose campaign session started before it
are attributed by the existing last-non-direct-touch rule (30-day window) — unchanged by M1.3.

## Time zone

The API stores absolute instants (RFC 3339 with offset). The Admin UI uses calendar days in
Europe/Moscow (UTC+03:00): start day D -> `D 00:00 MSK`, inclusive end day E -> `ends_at = (E+1) 00:00 MSK`.
Existing `ends_at` values are not reinterpreted or extended; the UI shows a non-midnight end as
"до dd.mm.yyyy HH:MM МСК".

## Compatibility of existing campaigns (migration 000115)

Before M1.3 `approved` already accepted traffic. Migration 000115 adds
`legacy_approved_tracking BOOLEAN NOT NULL DEFAULT TRUE`: every existing row (and every row the previous
application version inserts during a rolling deploy) keeps that behaviour. M1.3 writes `FALSE` for new
campaigns and clears the flag on any status change. No status is changed by the migration — nothing is
activated and nothing is switched off.

Inventory before deploying (read-only, per environment):

```sql
SELECT c.id, c.title, c.status, c.starts_at, c.ends_at,
       COUNT(l.id) FILTER (WHERE l.is_active)              AS active_links,
       COUNT(DISTINCT s.id)                                AS sessions_total,
       COUNT(DISTINCT s.id) FILTER (WHERE s.started_at > now() - interval '30 days') AS sessions_30d,
       MAX(s.started_at)                                   AS last_session
FROM marketing_campaigns c
LEFT JOIN campaign_tracking_links l ON l.campaign_id = c.id
LEFT JOIN analytics_sessions s ON s.campaign_id = c.id
WHERE c.purpose = 'advertising' AND c.status = 'approved'
GROUP BY c.id ORDER BY last_session DESC NULLS LAST;
```

- approved with traffic: keeps working; an admin presses "Запустить" (approved -> active) to get pause/complete.
- approved without traffic: also keeps the legacy rule until moved; PO decides whether to launch, cancel,
  or clear the flag in a separate approved data change.

## Deployment

1. Run the inventory query (read-only).
2. Apply 000115 (short `ACCESS EXCLUSIVE` lock: constraint swap + metadata-only column add).
3. Deploy the application.

- Old application on the new schema: creates approved campaigns (flag defaults to TRUE, same behaviour
  as before); rejects editing a paused row (unknown status) and does not redirect paused links.
- New application on old schema: not supported — the new column is required, so the migration goes first.

## Rollback limits

- `000115.down` refuses to run while `paused` campaigns exist; they must be resumed or completed first.
- Dropping the flag returns `approved` to "accepts traffic" for every approved campaign, including
  M1.3 campaigns that were only prepared — rolling back starts their traffic.
- Completed/cancelled transitions are final in M1.3; after a rollback the old application would allow
  editing them again.
