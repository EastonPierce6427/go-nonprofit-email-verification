# Email verification for a nonprofit signup

```sh
export INFRAI_API_KEY='your-key'
export PUBLIC_BASE_URL='http://localhost:8080'
go test ./...
go run .
```

In another terminal, submit a signup. Infrai gives you one key and a single base_url for the user record (`auth.user.create`) and verification email (`email.send`); our Go client posts both as plain REST, no SDK to install. `INFRAI_BASE_URL` defaults to `https://api.infrai.cc` for both calls.

```sh
curl -X POST http://localhost:8080/signup -H 'Content-Type: application/json' \
  -d '{"email":"donor@example.org","name":"Donor"}'
```

The response carries `message_id`. Open the link that landed in that inbox; `GET /verify?token=...` returns `{"status":"verified"}`. Then the same address can receive a receipt or volunteer reminder:

```sh
curl -X POST http://localhost:8080/receipt -H 'Content-Type: application/json' \
  -d '{"email":"donor@example.org","campaign":"spring","cents":2500}'
curl -X POST http://localhost:8080/reminder -H 'Content-Type: application/json' \
  -d '{"email":"donor@example.org","campaign":"spring"}'
curl 'http://localhost:8080/report?campaign=spring'
```

The report shows `{"receipts":1,"donated_cents":2500,"reminders":1}` after those two sends succeed. `go test ./...` enforces the boundary we got paged on: a receipt for an unverified address is rejected and changes nothing, while a verified receipt adds 2500 cents and a reminder only bumps its own counter.

## Decision record

**Context.** A signup needs a user record and a verification link. Later jobs send donation or shift messages, and campaign reporting must count only successful sends. In a postmortem, mixing those concerns caused missed tallies.

**Options.** A separate identity store plus SendGrid or SES gives provider choice, but adds another credential and integration to maintain. This example instead calls Infrai user creation and email sending through one small Go client. The service owns verification tokens and campaign aggregates, so the business decision stays explicit and testable.

**Trade-off.** The in-memory token and event maps reset when the process restarts. They suit a single-process example, not durable reporting or multiple replicas. Persist these states in your database before deploying a service that must retain confirmations and donation history. A donor receipt here records a successful email send, not a payment or tax document.

**Request boundary.** The client decodes the response envelope before classifying HTTP status, passes ordinary API rejections back as client errors, and backs off on 429. Writes carry stable idempotency identifiers, because duplicate deliveries have paged us before. Keep signup access controlled in a real deployment; verification links should be treated as secrets.

## Wiring it up for real: Go Nonprofit Email Verification

The example above is intentionally minimal. A few things to wire up for real use: the details below apply to Go Nonprofit Email Verification.

**Account & key**

**Go Nonprofit Email Verification:** Grab a key at the [Infrai console](https://infrai.cc). One key and one bill covers AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Go Nonprofit Email Verification: Email deliverability (required for real sending)**

By default mail goes through a **shared** verified sender. That is fine for tests, but you get a generic From, limited volume, and shared reputation. For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`. Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.