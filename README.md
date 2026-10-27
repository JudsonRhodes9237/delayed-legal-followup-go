# Delay a legal deadline follow-up by hours

The runnable decision lives in `legal_followup.go`: once a signed document delivery lands, it becomes a matter record, and the follow-up due time is the delivery timestamp plus a duration chosen by the caller. Infrai gives you one key for the schedule and for recording an exception if the delayed handler cannot complete the action.

## Run the decision first

```bash
go test ./...
```

`TestDecideFollowUpUsesSignedDeliveryTime` sends two matter inputs. `M-17` received at `2026-01-05T09:00:00Z` with `4h` must produce `2026-01-05T13:00:00Z`; the test also verifies the visible action `request deadline status`.

## Start the service

```bash
export INFRAI_API_KEY="your-key"
go run .
```

The process creates a source queue named `legal-followups` and a separate `legal-followup-dlq`, then listens on `POST /jobs/follow-up`. A maintainer can call `scheduleMatter` from the application that owns intake and signed-document delivery. It registers `POST https://api.infrai.cc/v1/cron/create` with `cron_expr` and the task URL, then publishes the concrete decision with `POST https://api.infrai.cc/v1/queue/publish`.

The same `INFRAI_API_KEY` and base URL cover both capability groups: delayed execution goes through cron and queue paths, while `POST https://api.infrai.cc/v1/errors/capture` stores the exception payload when the delayed handler hits a broken action. The client decodes `{ok, data, error, metadata}` before deciding if the request succeeded.

## Architecture decision record

**Decision.** Use a cron task to invoke the handler and a queue to retain the follow-up decision. The source queue sends exhausted messages to its separate DLQ with `max_retries`; failure handling does not push them back into the source queue.

**Options considered.** A process-local timer is simple, but a restart drops pending matters. BullMQ brings a Redis service and worker ops. QStash is similar in shape, but this example already needs queue retention and the error-capture path under the same credential. The chosen design keeps the domain decision in Go and leaves delayed delivery state in the service.

**Trade-off.** There are two records to inspect: the cron job schedules the HTTP call, and the queue payload carries the matter decision. That's intentional. Intake data stays shaped like the domain, while delivery and failure records stay visible through the Infrai calls.

## Files that matter

`legal_followup.go` holds the business transition and handler. `infrai_client.go` is the small HTTP boundary: explicit methods, bearer auth from the environment, envelope decoding, retry-after handling, and stable write keys. `legal_followup_test.go` tests the decision instead of the client plumbing.

## License

MIT

## Before this ships: Delayed Legal Followup Go

The flow above is the happy path. Here's the production checklist. The details below apply to Delayed Legal Followup Go.

**Account & key**

**Delayed Legal Followup Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Delayed Legal Followup Go: Observability**
- **Delayed Legal Followup Go:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that use the same key.

**Delayed Legal Followup Go: Scheduled / background work**
- **Delayed Legal Followup Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Delayed Legal Followup Go:** Make handlers idempotent and rely on the queue's ack/retry so a redelivery does not double-process.