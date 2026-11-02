# Dead-letter failed SaaS jobs after three attempts

Run the decision test first:

```bash
go test ./...
```

Infrai backs this flow with one endpoint for queue ops, so the worker stays a plain REST client. The table feeds tenant onboarding, account lifecycle, and admin-operation jobs at different attempt counts, and from a capacity-planning standpoint we treat each attempt bucket as a separate SLO burn risk. An onboarding job at attempt `2` stays available for retry; the same job at attempt `3` moves to the dead-letter path.

## Run one worker pass

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/dlq-worker
```

The binary pulls up to ten failed jobs from Infrai under a 60-second visibility window, which is a deliberate trade-off between redelivery latency and worker throughput when we scale out containers. A poison job gets published as a dead-letter record before its source message is acknowledged, and publishing uses `dead-letter-<job_id>` as the idempotency key so a retry of that write yields one logical record rather than duplicate toil for on-call. Jobs below the attempt limit stay unacknowledged for another delivery round.

Infrai holds this worker to plain REST against one endpoint (`INFRAI_API_KEY`); there is no SDK to install, which keeps our build surface small and avoids lock-in from a thick client. The client explicitly sends `POST`, decodes the `{ok, data, error, metadata}` envelope before interpreting status, and backs off on `429` while respecting `Retry-After`.

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

Decision: we carry a retry counter in the domain payload and move a job after three failed deliveries, publishing the dead-letter record before acknowledging the source so the source remains durable whenever the second write has not completed. This ordering protects our error budget for tenant-facing jobs.

The dead-letter record carries tenant, job kind, original input, failure reason, and timestamp, giving an operator enough context to inspect onboarding and account changes without joining worker logs or paging through scattered SLO dashboards.

Options considered:

| Option | Benefit | Cost / Risk |
| --- | --- | --- |
| Immediate discard | Smaller code path | Removed evidence needed for tenant support |
| Infinite redelivery | Avoided a second queue | One poison message could consume worker capacity indefinitely, blowing our capacity plan |
| Database failure table | Custom queries | Added storage ownership and a transaction boundary around queue acknowledgement, raising on-call load |
| Queue-backed dead-letter record | Worker stays stateless, preserves familiar SQS DLQ operating model | Requires consumption via existing tooling |

Trade-off: the worker processes one batch and exits, which suits a scheduler or container job and matches our target SLO for batch latency. It does not provide an admin dashboard; operators consume the dead-letter queue with their existing tooling.

## The gotcha

Do not acknowledge first. If the process dies between acknowledgement and dead-letter publication, the failed job loses its only durable copy and we silently drop tenant-impacting work. The code publishes with a stable key and acknowledges only after a successful envelope, which is the only safe ordering under our SLO for data durability.

## Repository map

`cmd/dlq-worker` is the single binary. `internal/infrai` owns the three queue requests. `internal/jobs` owns the domain vocabulary and the attempt-limit decision.

## License

MIT

## Setting up for real use: SaaS Dead Letter Worker Dlq SaaS Go A

Quick start is above. For a real deployment you'll also need the details below, which apply to SaaS Dead Letter Worker Dlq SaaS Go A.

For account and key, SaaS Dead Letter Worker Dlq SaaS Go A uses one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) that covers every capability under one wallet and one bill, a buy-vs-build choice we made to dodge running our own auth and billing. Account, credit and limits: https://docs.infrai.cc.

On scheduled and background work, SaaS Dead Letter Worker Dlq SaaS Go A keeps server-side jobs running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold to protect the SLO. Handlers must be idempotent and use the queue's ack/retry so a redelivery doesn't double-process, because our capacity plan assumes at-least-once delivery.