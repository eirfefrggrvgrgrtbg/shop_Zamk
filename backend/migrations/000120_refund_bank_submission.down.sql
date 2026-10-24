BEGIN;
DROP TRIGGER refunds_bank_attempt_immutable ON refunds;
DROP FUNCTION refunds_bank_attempt_immutable();
DROP INDEX refunds_retry_of_uniq;
DROP INDEX refunds_one_bank_attempt_per_return;
DROP INDEX refunds_idempotency_key_uniq;
ALTER TABLE refunds
    DROP CONSTRAINT refunds_submitted_has_key,
    DROP CONSTRAINT refunds_submit_outcome_matches_status,
    DROP CONSTRAINT refunds_submit_outcome_valid,
    DROP COLUMN retry_of_refund_id,
    DROP COLUMN review_reason,
    DROP COLUMN failure_code,
    DROP COLUMN provider_status,
    DROP COLUMN last_checked_at,
    DROP COLUMN submitted_at,
    DROP COLUMN submit_attempts,
    DROP COLUMN submit_outcome,
    DROP COLUMN idempotency_key;
COMMIT;
