# Security Policy

## Supported versions

The `main` branch is the supported line for security fixes.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security problems.

Email the maintainer via GitHub: [@foqerhk](https://github.com/foqerhk) (use a private vulnerability advisory on this repository when available).

Include:

- Affected commit or release
- Description and impact
- Minimal reproduction steps if possible

We aim to acknowledge reports within 7 days.

## Operational guidance

- Treat `.env`, APNs `.p8` keys, and LiveKit secrets as credentials
- Rotate secrets after any suspected leak
- Keep Amap Web keys on the server only
