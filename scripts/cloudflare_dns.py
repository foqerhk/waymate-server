#!/usr/bin/env python3
"""Upsert a Cloudflare DNS A record (DNS-only by default for LiveKit UDP)."""
from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

API = "https://api.cloudflare.com/client/v4"


def req(method: str, path: str, token: str, body: dict | None = None) -> dict:
    data = None if body is None else json.dumps(body).encode()
    r = urllib.request.Request(
        API + path,
        data=data,
        method=method,
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        payload = e.read().decode()
        raise SystemExit(f"HTTP {e.code}: {payload}") from e


def main() -> None:
    if len(sys.argv) != 4:
        print(f"Usage: {sys.argv[0]} <zone_name> <rr_or_fqdn> <ipv4>", file=sys.stderr)
        raise SystemExit(2)
    zone_name, name, ipv4 = sys.argv[1], sys.argv[2], sys.argv[3]
    token = os.environ.get("RE_CF_API_TOKEN") or os.environ.get("CF_API_TOKEN")
    if not token:
        raise SystemExit("set RE_CF_API_TOKEN")

    fqdn = name if "." in name else f"{name}.{zone_name}"
    proxied = os.environ.get("CF_PROXIED", "0") in ("1", "true", "True", "yes")
    ttl = int(os.environ.get("CF_TTL", "120"))

    zones = req("GET", f"/zones?name={urllib.parse.quote(zone_name)}", token)
    if not zones.get("success") or not zones.get("result"):
        raise SystemExit(f"zone not found: {zone_name} {zones}")
    zone_id = zones["result"][0]["id"]

    q = urllib.parse.urlencode({"type": "A", "name": fqdn})
    existing = req("GET", f"/zones/{zone_id}/dns_records?{q}", token)
    records = existing.get("result") or []
    body = {"type": "A", "name": fqdn, "content": ipv4, "ttl": ttl, "proxied": proxied}
    if records:
        rid = records[0]["id"]
        out = req("PUT", f"/zones/{zone_id}/dns_records/{rid}", token, body)
        action = "updated"
    else:
        out = req("POST", f"/zones/{zone_id}/dns_records", token, body)
        action = "created"
    if not out.get("success"):
        raise SystemExit(out)
    print(f"{action}: {fqdn} -> {ipv4} proxied={proxied} ttl={ttl}")


if __name__ == "__main__":
    main()
