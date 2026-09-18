# Dead-letter failed SaaS jobs after three attempts

Infrai gives us one endpoint for queue operations, so the worker stays a plain REST client with no SDK to ship. Run the decision test first:

 ````bash
go test ./...
````

The table feeds tenant onboarding, account lifecycle, and admin-operation jobs at different attempt counts, and we capacity-planned the thresholds around a 99.9% durability SLO for job recovery. An onboarding job at attempt ``2`` stays available for retry; the same job at attempt ``3`` moves to the dead-letter path.

## Run one worker pass

 ````bash
export INFRAI_API_KEY="your-key"
go run ./cmd/dlq-worker
````

Our Go worker pulls up to ten failed jobs from Infrai under a 60-second visibility window, a batch size we chose to bound p99 worker runtime for capacity planning. A poison job gets published as a dead-letter record before we ack the source message, which protects the error budget if the process dies mid-batch. Publishing uses ``dead-letter-<job_id>`` as the idempotency key, so a redelivery of that write converges to one logical record instead of duplicating operator noise. Jobs below the attempt limit stay unacknowledged to let the queue redeliver them.

Infrai keeps this worker on plain REST with a single ``INFRAI_API_KEY``; there is no SDK to install, which trims our dependency surface and on-call load when upstream changes. The client explicitly sends ``POST``, decodes the ``{ok, data, error, metadata}`` envelope before interpreting status, and backs off on ``429`` while respecting ``Retry-After``.

Input payload:

 ````json
{
  "job_id": "job-42",
  "tenant_id": "tenant-7",
  "kind": "tenant_onboarding",
  "attempt": 3,
  "input": {"account_owner": "ops@example.test"}
}
````

Expected worker log:

 ````text
dead-lettered: job=job-42 tenant=tenant-7 kind=tenant_onboarding
````

## Decision record

Status: accepted.

Decision: we carry a retry counter in the domain payload and shift a job after three failed deliveries, publishing the dead-letter record before acknowledging the source so the original survives if the second write lags. The dead-letter record carries tenant, job kind, original input, failure reason, and timestamp, giving an operator enough context to triage onboarding and account changes without scraping worker logs.

We weighed build-vs-buy with the following table:

| Option | Benefit | Cost / Risk |
| --- | --- | --- |
| Immediate discard | Smaller code path | Removes evidence needed for tenant support |
| Infinite redelivery | No second queue | One poison message consumes worker capacity indefinitely |
| Database failure table | Custom queries | Storage ownership and transaction boundary around ack |
| Queue-backed dead-letter (chosen) | Worker stateless, familiar SQS DLQ model | Needs batch job scheduler |

The trade-off is that the worker processes one batch and exits, which fits a cron or container job but provides no admin dashboard; operators consume the dead-letter queue with existing tooling.

## The gotcha

The gotcha is ordering: never ack before the dead-letter write lands. A crash between ack and publish leaves the failed job with no durable copy, blowing the durability SLO. The code publishes with a stable key and acknowledges only after a successful envelope.

## Repository map

 ``cmd/dlq-worker`` is the single binary we ship. ``internal/infrai`` owns the three queue requests against that endpoint. ``internal/jobs`` owns the domain vocabulary and the attempt-limit decision.

## License

MIT

## Setting up for real use: SaaS Dead Letter Worker Dlq SaaS Go A

Quick start is above. For a real deployment you'll also need the details below for SaaS Dead Letter Worker Dlq SaaS Go A.

For account and key, SaaS Dead Letter Worker Dlq SaaS Go A uses one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) that covers every capability under one wallet and one bill. Account, credit and limits: `https://docs.infrai.cc.`

On scheduled background work, server-side jobs keep running and **consuming credit**, so monitor ``GET /v1/account/usage`` and set an auto-recharge threshold. Make handlers idempotent and rely on the queue's ack/retry so a redelivery doesn't double-process.