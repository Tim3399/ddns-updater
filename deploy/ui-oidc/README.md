# Optional OIDC-protected UI

OIDC is intentionally kept outside the Go application. `oauth2-proxy` protects
the existing UI and can use Keycloak or another standards-compliant OIDC
provider.

The DDNS application itself is never published to the host. Only oauth2-proxy
is bound to `127.0.0.1:4180`, so an existing host Nginx can expose it over TLS.

## Keycloak example

Create a confidential OIDC client named `ddns-updater`.

- Issuer: `https://auth.example.com/realms/homelab`
- Redirect URI: `https://ddns.example.com/oauth2/callback`
- PKCE: S256
- Client authentication: enabled
- Standard scopes: `openid profile email`

Create the local files:

```sh
cp .env.example .env
cp secrets/config.json.example secrets/config.json
mkdir -p data

printf '%s' 'YOUR_OIDC_CLIENT_SECRET' > secrets/oidc-client-secret
dd if=/dev/urandom bs=32 count=1 2>/dev/null > secrets/cookie-secret

chmod 700 data secrets
chmod 600 .env secrets/*
chown -R 1000:1000 data secrets
```

Set `DDNS_IMAGE` to an immutable image reference, preferably an image digest.
For production, pin the oauth2-proxy image by digest as well.

```sh
docker compose config
docker compose up -d
```

Point the existing host Nginx to `127.0.0.1:4180` using
`nginx.conf.example`.

## Access restrictions

The default example accepts any authenticated email identity from the configured
OIDC issuer. For stricter access, configure an OIDC group claim in your provider
and add the matching oauth2-proxy `--allowed-group=<group>` option.

Do not publish port 8000 from `ddns-updater`. Authentication belongs in front of
the existing UI rather than in the DDNS Go core.
