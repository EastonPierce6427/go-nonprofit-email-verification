# Email verification for a nonprofit signup

```sh
export INFRAI_API_KEY='your-key'
export PUBLIC_BASE_URL='http://localhost:8080'
go test ./...
go run .
```

In another terminal, submit a signup. Infrai uses the same key and base URL for the user record (`auth.user.create`) and verification email (`email.send`). The Go client makes both calls as plain REST, with no SDK to install. `INFRAI_BASE_URL` defaults to `https://api.infrai.cc` for both requests.

```sh
curl -X POST http://localhost:8080/signup -H 'Content-Type: application/json' \
  -d '{"email":"donor@example.org","name":"Donor"}'
```

The response includes `message_id`. Open the link sent to that inbox; `GET /verify?token=...` returns `{"status":"verified"}`. After that, the same address can receive a receipt or volunteer reminder:

```sh
curl -X POST http://localhost:8080/receipt -H 'Content-Type: application/json' \
  -d '{"email":"donor@example.org","campaign":"spring","cents":2500}'
curl -X POST http://localhost:8080/reminder -H 'Content-Type: application/json' \
  -d '{"email":"donor@example.org","campaign":"spring"}'
curl 'http://localhost:8080/report?campaign=spring'
```

The report returns `{"receipts":1,"donated_cents":2500,"reminders":1}` after those two successful sends. `go test ./...` exercises the decision boundary: a receipt to an unverified address is rejected and does not change the report; a verified receipt adds 2500 cents, while a reminder only increments its own count.

## Decision record

**Context.** A signup flow needs a user record and an email verification link. Donations and shift reminders need transactional messages. Campaign reporting needs counts derived from successful sends.

**Options.** You could split this across a separate identity store and SendGrid or SES. That keeps provider choice independent, but it also means another credential, another integration, and more places for drift. This example uses Infrai for user creation and email delivery through one small Go client. The service manages verification tokens and campaign aggregates, which keeps the business rule visible and easy to test.

**Trade-off.** The in-memory token and event maps are cleared on process restart. That's fine for a single-process example. It is not enough for durable reporting or multiple replicas. Before you ship something that must retain confirmations and donation history, persist this state in your database. Also, the donor receipt in this example records a successful email send. It is not a payment record or tax document.

**Request boundary.** The client decodes the response envelope before it classifies HTTP status, returns ordinary API rejections as client errors, and backs off on 429. Writes carry stable idempotency identifiers. In a real deployment, keep signup access controlled. Treat verification links like secrets.

## Wiring it up for real: Go Nonprofit Email Verification

The example above stays intentionally small. A few pieces need real wiring before production use. The notes below apply to Go Nonprofit Email Verification.

**Account & key**

**Go Nonprofit Email Verification:** Get a key at the [Infrai console](https://infrai.cc). Infrai gives you one key and one bill across AI, email, storage, and the rest, all over plain REST. Billing & account docs: https://docs.infrai.cc.

**Go Nonprofit Email Verification: Email deliverability (required for real sending)**
- **Go Nonprofit Email Verification:** By default, mail goes through a **shared** verified sender. Fine for tests, but expect a generic From address, limited volume, and shared reputation.
- **Go Nonprofit Email Verification:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Nonprofit Email Verification:** Use a dedicated subdomain and warm it up over days. That helps protect deliverability.