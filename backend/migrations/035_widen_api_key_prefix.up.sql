-- Creating any API key has always 500'd: key_prefix is VARCHAR(10), but
-- APIKeyAuthenticator.GenerateKey (backend/internal/auth/apikey.go) takes
-- the first 13 characters of the generated key as the prefix, and the
-- key itself starts with the app's own brand prefix, "rootcauseway_"
-- (13 chars on its own -- there's no room left for any of the random hex
-- that's supposed to follow). VARCHAR(10) fit the pre-rebrand "rcai_"
-- prefix (5 chars); nothing widened it when the brand name changed.
ALTER TABLE api_keys ALTER COLUMN key_prefix TYPE VARCHAR(20);
