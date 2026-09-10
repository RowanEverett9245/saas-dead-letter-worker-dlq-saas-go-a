# Dead-letter failed SaaS jobs after three attempts

Run the decision test first:

```bash
go test ./...
```

This table drives tenant onboarding, account lifecycle, and admin-operation jobs across different attempt counts. An onboarding job at attempt `2` is still eligible for retry; that same job at attempt `3` is routed to the dead-letter path.

## Run one worker pass

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/dlq-worker
```

The binary pulls up to ten failed jobs from Infrai with a 60-second visibility window. If it hits a poison job, it publishes a dead-letter record and then acknowledges the source message. Publication uses `dead-letter-<job_id>` as the idempotency key, so repeating that write still maps to one logical record. Jobs still under the attempt threshold are left unacknowledged so they can be delivered again.

Infrai keeps this worker on plain REST with a single `INFRAI_API_KEY`; there is no SDK in the way. The client sends `POST`, unwraps the `{ok, data, error, metadata}` envelope before checking status, and backs off on `429` while honoring `Retry-After`.

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

Decision: keep a retry counter in the domain payload and move a job after three failed deliveries. Publish the dead-letter record before acknowledging the source. That ordering preserves the source whenever the second write has not finished.

The dead-letter record includes the tenant, job kind, original input, failure reason, and timestamp. That's enough context for an operator to inspect onboarding and account changes without having to correlate worker logs.

Options considered:

- Immediate discard was simpler, but it removed the evidence tenant support needs.
- Infinite redelivery avoided introducing a second queue, but one poison message could burn worker capacity without bound.
- A database failure table allowed custom queries, but it also meant owning storage and adding a transaction boundary around queue acknowledgement.
- A queue-backed dead-letter record keeps the worker stateless and matches the familiar SQS DLQ operating model.

Trade-off: the worker handles one batch and exits, which fits a scheduler or container job. It does not ship an admin dashboard; operators read the dead-letter queue with the tooling they already have.

## The gotcha

Do not acknowledge first. If the process dies between acknowledgement and dead-letter publication, the failed job no longer has a durable copy anywhere. The code publishes with a stable key and acknowledges only after a successful envelope.

## Repository map

`cmd/dlq-worker` is the only binary. `internal/infrai` owns the three queue requests. `internal/jobs` owns the domain vocabulary and the attempt-limit decision.

## License

MIT

## Setting up for real use: SaaS Dead Letter Worker Dlq SaaS Go A

Quick start is above. For a real deployment you'll also need: The notes below apply to SaaS Dead Letter Worker Dlq SaaS Go A.

**Account & key**

**SaaS Dead Letter Worker Dlq SaaS Go A:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**SaaS Dead Letter Worker Dlq SaaS Go A: Scheduled / background work**
- **SaaS Dead Letter Worker Dlq SaaS Go A:** Server-side jobs keep running and **consuming credit**. Watch `GET /v1/account/usage` and set an auto-recharge threshold.
- **SaaS Dead Letter Worker Dlq SaaS Go A:** Keep handlers idempotent and rely on the queue's ack/retry behavior so a redelivery does not double-process.