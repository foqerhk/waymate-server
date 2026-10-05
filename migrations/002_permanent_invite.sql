-- Permanent family invites: no practical expiry; high join cap.
UPDATE invite_codes
SET expires_at = '9999-12-31 23:59:59+00',
    max_uses = 1000000
WHERE expires_at < '9999-01-01';

ALTER TABLE invite_codes
    ALTER COLUMN max_uses SET DEFAULT 1000000;
