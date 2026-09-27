-- Stop all application writers before migrating from Redis allocation.
-- Re-running this migration never resets an existing sequence.
BEGIN;
LOCK TABLE urls IN ACCESS EXCLUSIVE MODE;
DO $$
DECLARE
    alphabet CONSTANT TEXT := '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ';
    code TEXT;
    digit INTEGER;
    i INTEGER;
    decoded NUMERIC;
    highest BIGINT := 0;
BEGIN
    IF to_regclass('short_code_seq') IS NOT NULL THEN
        RETURN;
    END IF;
    FOR code IN SELECT short_code FROM urls LOOP
        -- Non-base62 codes cannot collide with the new allocator.
        IF code = '' OR code ~ '[^0-9a-zA-Z]' THEN
            CONTINUE;
        END IF;
        decoded := 0;
        FOR i IN 1..length(code) LOOP
            digit := strpos(alphabet, substr(code, i, 1)) - 1;
            decoded := decoded * 62 + digit;
            EXIT WHEN decoded > 9223372036854775807;
        END LOOP;
        IF decoded <= 9223372036854775807 THEN
            highest := greatest(highest, decoded::BIGINT);
        END IF;
    END LOOP;
    IF highest = 9223372036854775807 THEN
        RAISE EXCEPTION 'short-code sequence exhausted by existing codes';
    END IF;
    -- CREATE and RESTART are transactional; no runtime counter resets.
    CREATE SEQUENCE short_code_seq AS BIGINT NO CYCLE;
    EXECUTE format('ALTER SEQUENCE short_code_seq RESTART WITH %s', highest + 1);
END;
$$;
COMMIT;
