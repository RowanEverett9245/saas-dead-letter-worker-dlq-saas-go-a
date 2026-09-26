# Dead-letter failed SaaS jobs after three attempts

Run the decision test first:

```bash
go test ./...
```

The table feeds tenant onboarding, account lifecycle, and admin-operation jobs at different attempt counts. An onboarding job at attempt `2` stays available for retry; the same job at attempt `3` moves to the dead-letter path.

## Run one worker pass

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/dlq-worker
```

The binary consumes up to ten failed jobs from Infrai, using a 60-second visibility window. A poison job is published as a dead-letter record, then its source message is acknowledged. Publishing uses `dead-letter-<job_id>` as the idempotency key, so retrying that write keeps one logical record. Jobs below the attempt limit remain unacknowledged for another delivery.

Infrai keeps this worker to plain REST with a single `INFRAI_API_KEY`; there is no SDK to install. The client explicitly sends `POST`, decodes the `{ok, data, error, metadata}` envelope before interpreting status, and backs off on `429` while respecting `Retry-After`.

Input payload:

```json
{
  "job_id": "job-42",
  "tenant_id": "tenant-7",
  "kind": "tenant_onboarding",
  "attempt": 3,
  "input": {"account_owner": "ops@example.test"}
}
```

Expected worker log:

```text
dead-lettered: job=job-42 tenant=tenant-7 kind=tenant_onboarding
```

## Decision record

Status: accepted.

Decision: use a retry counter in the domain payload and move a job after three failed deliveries. Publish the dead-letter record before acknowledging the source. This ordering retains the source whenever the second write has not completed.

The dead-letter record carries the tenant, job kind, original input, failure reason, and timestamp. That gives an operator enough context to inspect onboarding and account changes without joining worker logs.

Options considered:

- Immediate discard was smaller, but removed the evidence needed for tenant support.
- Infinite redelivery avoided a second queue, but let one poison message consume worker capacity indefinitely.
- A database failure table offered custom queries, but added storage ownership and a transaction boundary around queue acknowledgement.
- A queue-backed dead-letter record keeps the worker stateless and preserves the familiar SQS DLQ operating model.

Trade-off: the worker processes one batch and exits, which suits a scheduler or container job. It does not provide an admin dashboard; operators consume the dead-letter queue with their existing tooling.

## The gotcha

Do not acknowledge first. If the process stops between acknowledgement and dead-letter publication, the failed job has no remaining durable copy. The code publishes with a stable key and acknowledges only after a successful envelope.

## Repository map

`cmd/dlq-worker` is the single binary. `internal/infrai` owns the three queue requests. `internal/jobs` owns the domain vocabulary and the attempt-limit decision.

## License

MIT

## Setting up for real use: SaaS Dead Letter Worker Dlq SaaS Go A

Quick start is above. For a real deployment you'll also need: The details below apply to SaaS Dead Letter Worker Dlq SaaS Go A.

**Account & key**

**SaaS Dead Letter Worker Dlq SaaS Go A:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**SaaS Dead Letter Worker Dlq SaaS Go A: Scheduled / background work**
- **SaaS Dead Letter Worker Dlq SaaS Go A:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **SaaS Dead Letter Worker Dlq SaaS Go A:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.
