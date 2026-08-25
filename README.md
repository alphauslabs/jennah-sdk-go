# jennah-sdk-go

Go client library for the Jennah agent memory and context platform.

The `jennah/` package tree is generated from [jennah-api](https://github.com/alphauslabs/jennah-api/). For production use, pin to tagged releases.

## Installation

```bash
go get github.com/alphauslabs/jennah-sdk-go
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/alphauslabs/jennah-sdk-go"
)

func main() {
	ctx := context.Background()

	jc, err := jennah.NewClient(jennah.Config{})
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer jc.Close()

	// Optional: verify endpoint connectivity
	if err := jc.Ping(ctx); err != nil {
		log.Fatalf("ping failed: %v", err)
	}

	a := jc.Agent("agent-abc")
	_, err = a.Logs.Create(ctx, &jennah.ExecutionLogStep{
		StepId:         "step-1",
		ThoughtProcess: "deciding which tool to call",
	})
	if err != nil {
		log.Fatalf("failed to create log step: %v", err)
	}
}
```

## Connection

The SDK connects to the public gRPC endpoint `jennah-grpc.alphaus.cloud:443` (`jennah.DefaultEndpoint`). This is distinct from the HTTP gateway at `jennah.alphaus.cloud`. Leave `Config.Endpoint` empty to use the default gRPC endpoint.

Standard TLS transport credentials are used on port 443.

Credentials are sent in the `authorization: Bearer` metadata header on each RPC.

## Authentication and Credentials

`Config.APIKey` is optional. When omitted, the SDK resolves credentials in the following order:

1. `Config.Credentials` (a custom source such as a secret manager)
2. `Config.APIKey`
3. `JENNAH_API_KEY` environment variable
4. Stored CLI session from `jnh login` (`~/.config/jennah/credentials`)

```go
jc, err := jennah.NewClient(jennah.Config{})
if err != nil {
	log.Fatal(err)
}
log.Printf("authenticated with: %s", jc.Credential()) // e.g. "session from stored session"
```

`Client.Credential()` returns credential metadata and source without revealing secret values.

### Token Renewal

When using a CLI session token, the SDK automatically refreshes expired access tokens and reissues failed requests once:

- **Automatic token rotation**: Refreshed tokens are written back to `~/.config/jennah/credentials` so concurrent local processes stay synchronized.
- **API keys**: API keys do not renew. A rejected API key returns `credentials.ErrKeyRefused` (matched by `jennah.IsUnauthenticated`).

## Client API Overview

All platform services can be accessed from a single `Client` instance:

| Entry Point | Description |
|---|---|
| `Client.Spawn`, `Client.List`, `Client.Agent(id)` | Agent workspace lifecycle and management |
| `Agent.Memory` | Unified memory transport (`Commit`, `Query`, `Inspect`) |
| `Agent.Logs`, `Agent.Vectors`, `Agent.Graph` | Memory section helpers and `Graph.Supersede` |
| `Client.Datasets`, `Client.Dataset(id)` | Datasets with `Schema` and `Data` operations |
| `Client.Auth` | Identity, API keys, members, invitations, roles, and enterprise administration |
| `Client.Approvals` | Human approvals and approver allowlists |
| `Client.Billing` | Subscriptions, entitlements, and marketplace bindings |
| `Client.Platform` | Tenant resource locations |

Generated protobuf types and enums are aliased in the root package (`types.go`) for direct access without deep imports.

## Features

### Pagination

Iterators using Go range-over-func (`All`) automatically handle pagination across pages:

```go
for ds, err := range jc.Datasets.All(ctx, nil) {
	if err != nil {
		return err
	}
	use(ds)
}
```

Paging iterators are available across resources:
- `Client.All` (agents)
- `Approvals.All`, `Approvals.Approvers.All`
- `Auth.Keys.All`, `Auth.Members.All`, `Auth.Invitations.All`, `Auth.Roles.All`
- Inspect sections: `Vectors.AllChunks`, `Graph.AllNodes`, `Graph.AllEdges`, `Logs.AllSteps`

### Long-polling Approvals

`Approvals.WaitUntilDecided` blocks until an approval reaches a terminal state. It automatically handles intermediate 30-second server polling intervals:

```go
decision, err := jc.Approvals.WaitUntilDecided(ctx, approvalID)
```

Pass a context with a timeout or deadline to bound the wait duration.

### Retries

Retries use [gax-go](https://github.com/googleapis/gax-go) exponential backoff (AIP-4221 with jitter). `UNAVAILABLE` status codes are retried up to 3 times by default.

Retries are evaluated per request based on idempotency:

| Call | Retried When |
|---|---|
| `Memory.Commit` | No log section is present (vector and graph writes are idempotent; log steps are append-only) |
| `Data.Commit` | `IdempotencyKey` is set |
| `Approvals.Create` | `RequestKey` is set |
| Read operations | Always |
| Other write operations | Never |

Retries can be configured or disabled via `Config.Retry`.

### Error Classification

Helper functions classify gRPC errors returned by the platform:

- `jennah.IsAlreadyCommitted(err)`
- `jennah.IsLimitExceeded(err)`
- `jennah.IsUnsupported(err)`
- `jennah.IsDenied(err)`
- `jennah.IsUnauthenticated(err)`
- `jennah.IsNotFound(err)`
- `jennah.IsTransient(err)`
- `jennah.Code(err)` (maps gRPC status codes and context errors)

## Direct gRPC Connection

To access underlying gRPC service stubs directly, use `Client.Conn()`:

```go
import authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"

stub := authv1.NewAuthServiceClient(jc.Conn())
```

Stubs created with `Client.Conn()` automatically inherit the client's authentication interceptor and credentials.
