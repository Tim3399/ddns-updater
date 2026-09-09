# Deployment profiles

These examples are intentionally separate from the application core.

## `headless`

Recommended production profile for DDNS on an edge or infrastructure host.

- no published application ports
- read-only root filesystem
- non-root runtime
- all Linux capabilities dropped
- `no-new-privileges`
- provider configuration mounted read-only
- built-in backups disabled
- explicit HTTPS public-IP providers

Use this profile when no web UI is required.

## `ui-oidc`

Optional authenticated UI using a generic OIDC reverse-auth layer:

```text
Nginx/TLS -> oauth2-proxy -> ddns-updater
                 |
                 +-> Keycloak / generic OIDC provider
```

OIDC deliberately remains outside the Go application so the DDNS core does not
need to own sessions, OAuth callbacks, cookies, provider metadata, or identity
provider-specific behavior.

For production deployments, pin container images by immutable digest rather
than mutable tags such as `latest`.
