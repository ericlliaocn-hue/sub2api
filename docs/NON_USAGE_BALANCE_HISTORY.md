# Non-consumption history

The history endpoint merges legacy redeem records, affiliate transfers and wallet
events. Model charges and batch holds/captures/releases are excluded before paging.
A wallet mirror of a redeem record is suppressed by its source reference. New
user creation records the opening balance in the same transaction as the user.
Admin-created opening balances are identified separately from registration grants.

Deployment requires migration 234. It records subscription lifecycle and bonus
expiry changes, ignoring subscription usage counters. No historical balances are
changed by this migration.

## Historical recovery

Run `backend/scripts/preview-opening-balances.sql` read-only first. Candidate audit
IDs are time matches, not verified attribution. Check the email, request outcome,
exact granted amount and registration configuration applicable at creation. Do not
infer the opening grant from today's balance, today's defaults or retained usage
logs. Registration histories lacking authoritative evidence require manual review.
An approved backfill must insert into the existing wallet ledger with
`source_type=user_creation`, `source_id=<user ID>` and the original creation time,
and must not update users.balance. Record the evidence reference in metadata.

The existing aggregate remains redeem balances plus positive admin adjustments;
the UI labels it explicitly rather than presenting it as cash received. Separate
cash/bonus/admin totals and automated historical recovery remain outstanding.
