# jennah-sdk-go

The reference Go client for the Jennah agent memory & context platform.

The `jennah/` tree is generated from [jennah-api](https://github.com/alphauslabs/jennah-api/).
The main branch can be broken. Make sure to use tagged releases.

## Connect

The SDK speaks gRPC to the platform's public gRPC endpoint,
`jennah-grpc.alphaus.cloud:443` (`jennah.DefaultEndpoint`). That is **not** the
HTTP gateway on `jennah.alphaus.cloud`, which serves HTTP/JSON and cannot answer a
gRPC call. Leave `Endpoint` empty to get the right one.

TLS terminates at the load balancer on 443, so ordinary transport credentials are
all that is needed; there is no custom CA and no plaintext port to reach for.

```go
jc, err := jennah.NewClient(jennah.Config{}) // credential resolved; see below
if err != nil {
	return err
}
defer jc.Close()

// Optional: force the lazy connection open and confirm the endpoint is serving.
if err := jc.Ping(ctx); err != nil {
	return err
}

a := jc.Agent("agent-abc")
_, err = a.Logs.Create(ctx, &jennah.ExecutionLogStep{
	StepId:         "step-1",
	ThoughtProcess: "deciding which tool to call",
})
```

The credential goes on the `authorization: Bearer` metadata header of every call.
The SDK's interceptor attaches it per RPC, since dialing carries no metadata.

## Credentials

`Config.APIKey` is optional. Left empty, the client resolves one itself, taking
the first source that answers and consulting no further:

| Order | Source |
|---|---|
| 1 | `Config.Credentials`, a source you supply (a secret manager, a test double) |
| 2 | `Config.APIKey` |
| 3 | `$JENNAH_API_KEY` |
| 4 | the session stored by `jnh login`, at `~/.config/jennah/credentials` |

A developer who has logged in with the CLI can therefore write
`jennah.NewClient(jennah.Config{})` and be authenticated as themselves. A
deployed service that sets a key is unaffected: because a later source is never
read, a machine with no session file cannot fail a caller who supplied one.

```go
jc, _ := jennah.NewClient(jennah.Config{})
log.Printf("authenticated with %s", jc.Credential()) // "session from stored session"
```

`Client.Credential` reports what the credential is and where it came from, never
its value, so it is safe to log.

Two things this deliberately does not do. It does not adopt the endpoint recorded
inside a stored session: that field names the front door the session was obtained
through, which for a CLI-written session is the HTTP gateway, and that hostname
cannot answer a gRPC call. And it does not treat the session's recorded expiry as
authoritative in the client's favor — a session that cannot be renewed and has
expired fails at construction with an actionable error, but a session that merely
looks valid is still the platform's to accept or reject.

### Renewal

A resolved session renews itself. When the platform rejects the access token, the
client refreshes it and reissues the call exactly once; a second rejection is
returned. So a long-running program keeps working across an expiry without
handling one, and there is nothing to schedule or refresh ahead of time.

Three properties are worth knowing about:

- **The reissue needs no idempotency key.** An `UNAUTHENTICATED` rejection is
  decided before the call reaches the operation, so it had no effect and cannot
  be applied twice. This is why a `data:commit` with no `IdempotencyKey` is
  reissued here even though the transport retry would refuse to replay it.
- **A renewal is written back to `~/.config/jennah/credentials`.** Refreshing
  rotates the refresh token, so a client that renewed privately would leave every
  other reader of that file — the CLI included — holding one the platform will
  never accept again. Concurrent calls renew once, and a renewal that fails
  because another process rotated first adopts what that process wrote rather
  than reporting a dead session.
- **An API key is never renewed.** It has no refresh token, so a rejection means
  the key itself was refused. The error says so (`credentials.ErrKeyRefused`)
  rather than pointing at a login that would not help, and still reads as
  `jennah.IsUnauthenticated`.

## Surface

Every service the endpoint publishes is reachable from one `Client`:

| Entry point | Covers |
|---|---|
| `Client.Spawn`, `Client.List`, `Client.Agent(id)` | agent workspaces |
| `Agent.Memory` | `Commit`, `Query`, `Inspect` (the unified memory transport) |
| `Agent.Logs`, `Agent.Vectors`, `Agent.Graph` | single-section wrappers over the above, plus `Graph.Supersede` |
| `Client.Datasets`, `Client.Dataset(id)` | application datasets; the handle carries `Schema` and `Data` |
| `Client.Auth` | `WhoAmI`, plus `Session`, `Keys`, `Members`, `Invitations`, `Roles`, `Enterprise` |
| `Client.Approvals` | human approvals, plus the `Approvers` allowlist |
| `Client.Billing` | subscription and entitlement state, marketplace binding |
| `Client.Platform` | where tenant resources can live |

Agents own the bare top-level verbs because they are the platform's primary object
and had them first. Everything else is namespaced, since one `Client.List` cannot
mean agents, datasets, approvals, and keys at once.

Every generated message and enum is aliased into the root package (`types.go`), so
building a request needs no deep import.

## Behavior the wrappers add

These are the parts worth having a client library for. Each one encodes a server
contract a caller would otherwise have to discover the hard way.

**Waiting for a human.** `Approvals.WaitUntilDecided` blocks until the approval is
terminal. The server caps one `WaitApproval` at 30 seconds and reports a reached
ceiling as a *success* carrying the still-pending approval with `TimedOut` set, so
code that treats a returned approval as a decision acts on a pending one. Bound it
with the context; with no deadline it waits as long as the human takes.

**Paging.** `All` iterators walk every page, so cursors stay out of your code:

```go
for ds, err := range jc.Datasets.All(ctx, nil) {
	if err != nil {
		return err
	}
	use(ds)
}
```

The same for `Client.All` (agents), `Approvals.All`, `Auth.Keys.All`,
`Auth.Members.All`, `Auth.Invitations.All`, `Auth.Roles.All`,
`Approvals.Approvers.All`, and one per inspect section: `Vectors.AllChunks`,
`Graph.AllNodes`, `Graph.AllEdges`, `Logs.AllSteps`. Breaking out stops fetching,
and your request is never mutated.

**Retries, decided per request rather than per method.** The loop is
[gax](https://github.com/googleapis/gax-go)'s `Invoke`, the same one Google's
generated clients use, so the backoff is the AIP-4221 shape: jittered, growing by
two, capped. `UNAVAILABLE` is retried three times by default.

gax deliberately has no attempt cap, on the reasoning that a deadline bounds a loop
better than a count. This SDK adds one anyway, because an agent commonly runs with
no deadline at all and an uncapped loop against an unreachable endpoint would never
return. A deadline reached during a backoff ends the call with the context's error,
not the last transport failure, which is why `Code` maps context errors instead of
calling them `Unknown`.

Reads always qualify for replay. Writes qualify only when a replay cannot produce a
second effect:

| Call | Replayed when |
|---|---|
| `Memory.Commit` | there is no log section (vector and graph writes are idempotent upserts; log steps are append-only) |
| `Data.Commit` | `IdempotencyKey` is set |
| `Approvals.Create` | `RequestKey` is set |
| everything else that writes | never |

That distinction is why this lives in the SDK: a gRPC service-config policy sees
the method but never the request. Turn it off with `Config.Retry.Disabled`, and see
`TestEveryMethodIsClassified`, which fails if a new RPC is left unclassified.

**Error classification.** `IsAlreadyCommitted`, `IsLimitExceeded`, `IsUnsupported`,
`IsDenied`, `IsUnauthenticated`, `IsNotFound`, `IsTransient`, and `Code` for the
rest. A re-committed log step returns `AlreadyExists`, which an agent retrying after
an ambiguous failure usually wants to read as success.

## Escape hatch

For anything the wrappers do not cover, `Client.Conn()` returns the live
connection. Stubs built on it are already credentialed, because the bearer is
attached by a dial-time interceptor rather than by the wrappers:

```go
stub := authv1.NewAuthServiceClient(jc.Conn())
```

## Authority

Both front doors terminate on the same server behind the same authentication and
authorization chain. A credential gets the same decision on the same operation
whichever one it arrives at. In particular, the enterprise-administration calls
under `Client.Auth` refuse an API key over gRPC exactly as they do over HTTP,
because a key can never hold a management-class scope. Transport selects which
credential you may present, never what it may do.
