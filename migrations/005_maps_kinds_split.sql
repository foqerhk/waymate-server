-- Split LBS metering into walk / transit (keep legacy 'lbs' rows).

ALTER TABLE maps_usage_daily DROP CONSTRAINT IF EXISTS maps_usage_daily_kind_check;
ALTER TABLE maps_usage_daily
  ADD CONSTRAINT maps_usage_daily_kind_check
  CHECK (kind IN ('search', 'lbs', 'walk', 'transit'));
