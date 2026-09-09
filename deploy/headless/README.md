# Hardened headless deployment

This is the recommended production profile for an edge VM. The application
opens no inbound application port and only needs outbound HTTPS/DNS access.

## Setup

```sh
cp .env.example .env
cp secrets/config.json.example secrets/config.json
mkdir -p data

chmod 700 data secrets
chmod 600 .env secrets/config.json
# Match the image runtime user when bind-mount permissions require it.
chown -R 1000:1000 data secrets
```

Edit `secrets/config.json`, set a dedicated/provider-scoped token and use a
single-purpose DDNS record such as `home.example.com`.

Set `DDNS_IMAGE` to an immutable image digest, not `latest`.

```sh
docker compose config
docker compose up -d
docker compose logs --tail=100 ddns-updater
```

## Security properties

- no published ports
- non-root UID/GID 1000
- read-only root filesystem
- all Linux capabilities dropped
- `no-new-privileges`
- provider config mounted read-only
- restrictive `UMASK=0077`
- info logging only
- built-in application backups disabled
- small explicit HTTPS public-IP provider set

Do not enable debug logging on an unpatched upstream image because credentials
can be included in HTTP/config debug output.
