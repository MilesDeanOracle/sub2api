-- Serialize refund review with claims in the same user-first lock order.
CREATE OR REPLACE FUNCTION activity_consumption_refund_review() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.order_type='balance' AND NEW.status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED','REFUND_FAILED') AND OLD.status IS DISTINCT FROM NEW.status THEN
  -- Serialize refund eligibility changes with claims and settlement (user first).
  PERFORM 1 FROM users WHERE id=NEW.user_id FOR UPDATE;
  INSERT INTO activity_refund_reviews(order_id,user_id,order_status) VALUES(NEW.id,NEW.user_id,NEW.status)
  ON CONFLICT(order_id) DO UPDATE SET order_status=EXCLUDED.order_status,review_status='pending',updated_at=NOW();
  UPDATE activity_consumption_entitlements SET state='review' WHERE user_id=NEW.user_id AND state='reserved';
 END IF;
 RETURN NEW;
END $$;
