-- accounts / pubkeys / tokens, with RLS scoped by the app.current_account GUC.
--
-- PRODUCTION COMMITMENT: the schema-owner/migrate role MUST be a non-superuser
-- in production. FORCE ROW LEVEL SECURITY ensures even the owner is subject to
-- RLS policies. The dev docker-compose owner stays superuser — that is fine for
-- local/CI only.

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

-- FORCE RLS so even the table owner (when running as a non-superuser in
-- production) cannot bypass policies. Superusers still bypass FORCE RLS, so
-- this is defence-in-depth: it kicks in the moment the owner is demoted.
ALTER TABLE accounts      FORCE ROW LEVEL SECURITY;
ALTER TABLE ssh_pubkeys   FORCE ROW LEVEL SECURITY;
ALTER TABLE swiggy_tokens FORCE ROW LEVEL SECURITY;

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
-- SET search_path = '' (empty) prevents pg_temp shadowing attacks: the definer
-- runs as a superuser, so without a pinned search_path an attacker holding the
-- broker DSN could create pg_temp.accounts to hijack the function body.
-- ON CONFLICT ... DO UPDATE makes the insert race-safe for concurrent signups.
CREATE OR REPLACE FUNCTION find_or_create_account(p_phone text)
    RETURNS uuid
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = ''
AS $$
DECLARE
    v_id uuid;
BEGIN
    INSERT INTO public.accounts (phone)
        VALUES (p_phone)
        ON CONFLICT (phone) DO UPDATE SET phone = EXCLUDED.phone
        RETURNING id INTO v_id;
    RETURN v_id;
END;
$$;

-- Pubkey -> account lookup happens before current_account is known, so it is a
-- SECURITY DEFINER function returning only the owning account id (no token data).
-- SET search_path = '' pins the search path so pg_temp.ssh_pubkeys cannot
-- shadow the real table in the definer's elevated context.
CREATE OR REPLACE FUNCTION account_for_pubkey(p_pubkey text)
    RETURNS uuid
    LANGUAGE sql
    SECURITY DEFINER
    SET search_path = ''
AS $$
    SELECT account_id FROM public.ssh_pubkeys WHERE pubkey = p_pubkey;
$$;

-- Broker role: a NON-owner login role, so RLS is enforced against it.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'console_broker') THEN
        CREATE ROLE console_broker LOGIN PASSWORD 'console_broker_dev';
    END IF;
END$$;

GRANT USAGE ON SCHEMA public TO console_broker;
GRANT SELECT, INSERT, UPDATE, DELETE ON accounts, ssh_pubkeys, swiggy_tokens TO console_broker;
GRANT EXECUTE ON FUNCTION find_or_create_account(text) TO console_broker;
GRANT EXECUTE ON FUNCTION account_for_pubkey(text) TO console_broker;
