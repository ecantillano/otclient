# Production endpoint transport check

Date: 2026-07-21

Command:

```bash
curl --max-redirs 0 --connect-timeout 10 --max-time 20 https://login.thappy.cl/login.php
openssl s_client -connect login.thappy.cl:443 -servername login.thappy.cl -verify_return_error
```

Observed:

```text
remote_ip=200.74.106.193
ssl_verify_result=0
certificate_subject=CN=login.thappy.cl
certificate_issuer=Let's Encrypt YE2
certificate_not_before=2026-07-18T22:25:51Z
certificate_not_after=2026-10-16T22:25:50Z
subject_alt_name=DNS:login.thappy.cl
```

A plain unauthenticated GET returned HTTP 404 with no redirect. This does not invalidate the canonical profile because Thappy deliberately uses `httpLogin=false`; the client uses the native login flow on port 443. The check proves DNS, TCP, TLS hostname validation and certificate chain at the time shown. It does not prove character list or game login.

No account, password, token or session was sent or logged.
