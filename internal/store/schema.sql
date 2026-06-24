-- accounts / pubkeys / tokens, with RLS scoped by the app.current_account GUC.

CREATE TABLE IF NOT EXISTS accounts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    phone      text UNIQUE NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ssh_pubkeys (
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    pubkey     text NOT NULL,
    PRIMARY KEY (account_id, pubkey)
);

CREATE TABLE IF NOT EXISTS swiggy_tokens (
    account_id  uuid PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    ciphertext  bytea NOT NULL,
    nonce       bytea NOT NULL,
    dek_wrapped bytea NOT NULL,
    expires_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE accounts      ENABLE ROW LEVEL SECURITY;
ALTER TABLE ssh_pubkeys   ENABLE ROW LEVEL SECURITY;
ALTER TABLE swiggy_tokens ENABLE ROW LEVEL SECURITY;

-- Scope every row to the account id in the app.current_account GUC.
DROP POLICY IF EXISTS acct_isolation ON accounts;
CREATE POLICY acct_isolation ON accounts
    USING (id = current_setting('app.current_account', true)::uuid);

DROP POLICY IF EXISTS pk_isolation ON ssh_pubkeys;
CREATE POLICY pk_isolation ON ssh_pubkeys
    USING (account_id = current_setting('app.current_account', true)::uuid)
    WITH CHECK (account_id = current_setting('app.current_account', true)::uuid);

DROP POLICY IF EXISTS tok_isolation ON swiggy_tokens;
CREATE POLICY tok_isolation ON swiggy_tokens
    USING (account_id = current_setting('app.current_account', true)::uuid)
    WITH CHECK (account_id = current_setting('app.current_account', true)::uuid);

-- Signup must create/lookup an account before any current_account is set, so it
-- runs as a SECURITY DEFINER (owner) function that bypasses RLS for this one op.
CREATE OR REPLACE FUNCTION find_or_create_account(p_phone text)
    RETURNS uuid
    LANGUAGE plpgsql
    SECURITY DEFINER
AS $$
DECLARE
    v_id uuid;
BEGIN
    SELECT id INTO v_id FROM accounts WHERE phone = p_phone;
    IF v_id IS NULL THEN
        INSERT INTO accounts (phone) VALUES (p_phone) RETURNING id INTO v_id;
    END IF;
    RETURN v_id;
END;
$$;

-- Pubkey -> account lookup happens before current_account is known, so it is a
-- SECURITY DEFINER function returning only the owning account id (no token data).
CREATE OR REPLACE FUNCTION account_for_pubkey(p_pubkey text)
    RETURNS uuid
    LANGUAGE sql
    SECURITY DEFINER
AS $$
    SELECT account_id FROM ssh_pubkeys WHERE pubkey = p_pubkey;
$$;

-- Broker role: a NON-owner login role, so RLS is enforced against it.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'console_broker') THEN
        CREATE ROLE console_broker LOGIN PASSWORD 'console_broker_dev';
    END IF;
END$$;

GRANT SELECT, INSERT, UPDATE, DELETE ON accounts, ssh_pubkeys, swiggy_tokens TO console_broker;
GRANT EXECUTE ON FUNCTION find_or_create_account(text) TO console_broker;
GRANT EXECUTE ON FUNCTION account_for_pubkey(text) TO console_broker;
