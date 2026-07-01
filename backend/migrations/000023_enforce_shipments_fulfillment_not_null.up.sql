BEGIN;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 FROM shipments WHERE fulfillment_id IS NULL) THEN
        RAISE EXCEPTION 'Cannot enforce NOT NULL: NULL shipments.fulfillment_id exist.';
    END IF;

    IF EXISTS (SELECT 1 FROM shipments s LEFT JOIN order_fulfillments of ON s.fulfillment_id = of.id WHERE s.fulfillment_id IS NOT NULL AND of.id IS NULL) THEN
        RAISE EXCEPTION 'Cannot enforce NOT NULL: orphan shipments.fulfillment_id exist.';
    END IF;

    IF EXISTS (SELECT 1 FROM shipments s JOIN order_fulfillments of ON s.fulfillment_id = of.id WHERE s.order_id != of.order_id) THEN
        RAISE EXCEPTION 'Cannot enforce NOT NULL: shipment order_id mismatch exist.';
    END IF;
END $$;

ALTER TABLE shipments ALTER COLUMN fulfillment_id SET NOT NULL;

COMMIT;
