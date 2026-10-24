BEGIN;

-- BANK.REFUND.0: the bank side of a refund is tracked next to its public status, which stays
-- pending / succeeded / failed (refunds.status has no CHECK; processing / completed are also read by the code).
-- submit_outcome is internal:
--   legacy     the row existed before this migration: its result (or reservation) is history, it was never handed
--              to a bank provider by ZAMK and is never handed to one now
--   not_sent   reserved after this migration, not handed to a bank provider yet
--   sending    the attempt is durably recorded and the provider call may be in flight
--   accepted   the provider took the request; the money movement is not confirmed yet
--   unknown    the call ended without a usable answer (timeout, transport error)
--   confirmed  the provider confirmed the refund of exactly this attempt (status succeeded)
--   rejected   the provider definitively refused this attempt (status failed)
-- While sending / accepted / unknown the row stays status 'pending', so every existing reservation query keeps
-- the amount reserved.

-- Every existing row becomes 'legacy' (the ADD COLUMN default); rows created afterwards default to 'not_sent'.
ALTER TABLE refunds ADD COLUMN submit_outcome TEXT NOT NULL DEFAULT 'legacy';
ALTER TABLE refunds ALTER COLUMN submit_outcome SET DEFAULT 'not_sent';

ALTER TABLE refunds
    ADD COLUMN idempotency_key TEXT,
    ADD COLUMN submit_attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN submitted_at TIMESTAMPTZ,
    ADD COLUMN last_checked_at TIMESTAMPTZ,
    ADD COLUMN provider_status TEXT,
    ADD COLUMN failure_code TEXT,
    ADD COLUMN review_reason TEXT,
    ADD COLUMN retry_of_refund_id UUID REFERENCES refunds(id) ON DELETE SET NULL;

ALTER TABLE refunds ADD CONSTRAINT refunds_submit_outcome_valid
    CHECK (submit_outcome IN ('legacy', 'not_sent', 'sending', 'accepted', 'unknown', 'confirmed', 'rejected'));
-- legacy / not_sent carry any historical status; a bank attempt's outcome must agree with the status.
ALTER TABLE refunds ADD CONSTRAINT refunds_submit_outcome_matches_status
    CHECK (submit_outcome IN ('legacy', 'not_sent')
        OR (submit_outcome IN ('sending', 'accepted', 'unknown') AND status = 'pending')
        OR (submit_outcome = 'confirmed' AND status = 'succeeded')
        OR (submit_outcome = 'rejected' AND status = 'failed'));
ALTER TABLE refunds ADD CONSTRAINT refunds_submitted_has_key
    CHECK (submit_outcome IN ('legacy', 'not_sent') OR idempotency_key IS NOT NULL);

CREATE UNIQUE INDEX refunds_idempotency_key_uniq ON refunds (idempotency_key) WHERE idempotency_key IS NOT NULL;
-- At most one bank attempt of a return is unresolved at a time.
CREATE UNIQUE INDEX refunds_one_bank_attempt_per_return ON refunds (return_id)
    WHERE return_id IS NOT NULL AND submit_outcome IN ('sending', 'accepted', 'unknown');
-- A definitively failed attempt is retried at most once; the retry points at it.
CREATE UNIQUE INDEX refunds_retry_of_uniq ON refunds (retry_of_refund_id) WHERE retry_of_refund_id IS NOT NULL;

-- Bank attempt invariants the application cannot bypass:
--   * 'legacy' is only the migration's own marking: no row is inserted or updated into it, none leaves it, and a
--     settled legacy row (succeeded / completed / failed) keeps its status and amount
--   * the key, payment and amount never change once the key exists
--   * an attempt only moves forward: not_sent -> sending -> accepted / unknown -> confirmed / rejected; a confirmed /
--     rejected outcome is reachable only from an unresolved attempt and is final
CREATE FUNCTION refunds_bank_attempt_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.submit_outcome = 'legacy' THEN
            RAISE EXCEPTION 'new refunds cannot be legacy' USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;
    IF OLD.submit_outcome = 'legacy' AND NEW.submit_outcome IS DISTINCT FROM 'legacy' THEN
        RAISE EXCEPTION 'a legacy refund is never handed to a bank provider' USING ERRCODE = '23514';
    END IF;
    IF OLD.submit_outcome = 'legacy' AND OLD.status IN ('succeeded', 'completed', 'failed') AND (
           NEW.status IS DISTINCT FROM OLD.status OR NEW.amount_cents IS DISTINCT FROM OLD.amount_cents) THEN
        RAISE EXCEPTION 'a settled legacy refund is history and cannot be rewritten' USING ERRCODE = '23514';
    END IF;
    IF OLD.submit_outcome <> 'legacy' AND NEW.submit_outcome = 'legacy' THEN
        RAISE EXCEPTION 'a refund cannot become legacy' USING ERRCODE = '23514';
    END IF;
    IF OLD.idempotency_key IS NOT NULL AND (
           NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key
        OR NEW.amount_cents IS DISTINCT FROM OLD.amount_cents
        OR NEW.payment_id IS DISTINCT FROM OLD.payment_id) THEN
        RAISE EXCEPTION 'refund idempotency key, payment and amount are immutable' USING ERRCODE = '23514';
    END IF;
    IF NEW.submit_outcome IS DISTINCT FROM OLD.submit_outcome AND NOT (
           (OLD.submit_outcome = 'not_sent' AND NEW.submit_outcome = 'sending')
        OR (OLD.submit_outcome = 'sending' AND NEW.submit_outcome IN ('accepted', 'unknown', 'confirmed', 'rejected'))
        OR (OLD.submit_outcome IN ('accepted', 'unknown') AND NEW.submit_outcome IN ('accepted', 'confirmed', 'rejected'))) THEN
        RAISE EXCEPTION 'refund bank outcome cannot move from % to %', OLD.submit_outcome, NEW.submit_outcome USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER refunds_bank_attempt_immutable
    BEFORE INSERT OR UPDATE ON refunds
    FOR EACH ROW EXECUTE FUNCTION refunds_bank_attempt_immutable();

COMMIT;
