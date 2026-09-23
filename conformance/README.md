# Client conformance suites

Language-independent cases that every published SDK must pass before it is
released. The cases are defined once, here, next to the proto they accompany;
each SDK carries a small harness that reads them and drives its own client
through them. No SDK keeps its own interpretation of a contract that a suite
here covers.

CI copies this directory into every generated SDK alongside the generated code,
so an SDK always runs the cases from the same revision its stubs came from.

## Rules

- **A new behavior enters the suite before it enters any SDK.** Add the case
  first, watch it fail in every SDK that lacks the behavior, then implement.
  A suite that trails its implementations only certifies that they have not
  changed recently.
- **A failing case is a defect in the SDK, not in the case.** Fix the client.
  Change a case only when the spec it cites changes, and cite the change.
- **Harnesses translate, they do not interpret.** A harness maps each op and
  each expectation onto its SDK's API mechanically. If a case cannot be
  expressed without judgment, the case is underspecified: fix it here.
- **Unknown ops, unknown expectation keys, and unknown `requires` values fail
  the case.** A harness that silently skips what it does not understand passes
  suites it never ran.

## `credentials/cases.json`

Derived from `client-credentials` (jennah `openspec/specs/client-credentials/spec.md`).
Each case cites the requirement and scenario it tests.

### Isolation

Every case runs on an empty machine: a fresh per-user config directory (so the
stored session is at `<config>/jennah/credentials`) and no `JENNAH_API_KEY` in
the environment unless the case sets one. A harness that let the developer's
real session or key leak in would pass or fail on whether its runner is logged
in.

### `given`

| Key | Meaning |
|---|---|
| `explicit` | Credential the program supplies in its client configuration. Absent means none. |
| `env` | Value of `JENNAH_API_KEY`. Absent means unset. |
| `session` | Stored session written before the case starts. Absent means no file. A `{"raw": "..."}` object is written verbatim; any other object is a session (see below). |
| `server` | The fake platform the client talks to (see below). |

A **session object** has the file's fields `endpoint`, `access_token`,
`refresh_token`, `token_type`, plus `expires_in`: seconds relative to now,
written to the file as `expires_at = now + expires_in` in unix seconds.
Absent `expires_in` writes `expires_at: 0` (unknown). Sessions are written in the
canonical format (see `round_trip`).

### The fake platform

The harness runs an in-process gRPC server and points the client at it, unless
a `construct` step says otherwise. It implements two business methods and the
renewal method:

| Method name in cases | RPC | Why this one |
|---|---|---|
| `read` | `jennah.agent.v1.AgentService/ListAgents` | Safe to replay after a transport failure. |
| `write_unsafe` | `jennah.datastore.v1.DataService/CommitData` with no `idempotency_key` | Not safe to replay after a transport failure. |
| (renewal) | `jennah.auth.v1.AuthService/RefreshToken` | The client's own renewal. |

`server` keys:

| Key | Meaning |
|---|---|
| `accept` | The bearer business methods accept. Anything else is `UNAUTHENTICATED`. Absent accepts nothing. |
| `reject_all` | Business methods reject every bearer, including a freshly renewed one. |
| `unavailable_first` | The first N business attempts fail `UNAVAILABLE` before authentication is checked. |
| `refresh` | `{accept, access_token, refresh_token, expires_in}`. A renewal presenting `accept` succeeds and **rotates**: the server's `accept` becomes `access_token` and `refresh.accept` becomes `refresh_token`, so the refresh token just spent is dead. Any other refresh token is `UNAUTHENTICATED`. Absent refuses every renewal. |
| `on_refresh_write_session` | A session object the server writes to the stored session file when a renewal arrives, before answering. It models another process winning a rotation race. |

The server records, in order, the bearer presented on every business attempt
(`presented`) and on every renewal attempt (`refresh_bearers`, `""` when none),
and counts renewal attempts (`refresh_calls`) and successful rotations
(`refreshes`).

### Steps

| Op | Does | `expect` |
|---|---|---|
| `construct` | Build a client from `explicit` (or none). With `"endpoint": "default"`, build it with no endpoint configured and no fake server. | `{"ok": true}` or an error expectation |
| `describe` | Read what the client reports about its credential and endpoint. The harness also asserts that no token from `given` appears anywhere in the rendered report. | Any of `kind` (`api_key`, `session`), `origin` (`explicit`, `environment`, `file`), `endpoint` |
| `call` | Invoke `method` once through the client. | `{"ok": true}` or an error expectation |
| `call_concurrently` | Invoke `method` `count` times at once. The server holds its answer to each of the next `count` business attempts until all `count` have arrived, so every call is in flight with the same credential before any is rejected. Without that barrier a fast transport lets the first call finish renewing before the others start, and the case passes without testing anything. | `{"ok": true}` means every call succeeded |
| `write_session` | Another process replaces the stored session with `session`. | none |
| `lock_session_directory` | Make the stored session's directory unwritable. Only in cases that declare `"requires": ["unwritable_directory"]`. | none |
| `atomic_replace` | Through the SDK's own session save and load, rewrite the stored session `writes` times with distinct contents while concurrently loading it. | `partial_reads`: loads that failed to parse or returned a session no writer wrote |
| `round_trip` | Write `raw` verbatim, load it through the SDK, save it back through the SDK. | `identical`: the file's bytes are unchanged |
| `check` | Assert final state. | see below |

An **error expectation** is `error`: categories that must all hold, `not`:
categories that must not hold, and `mentions`: substrings the error message
must contain (`$SESSION_PATH` is the stored session's path). Categories:

| Category | Holds when |
|---|---|
| `no_credential` | No source yielded a credential. |
| `corrupt_session` | The stored session exists and cannot be interpreted. |
| `session_expired` | The session cannot authenticate and cannot be renewed; the fix is to log in again. |
| `credential_refused` | A non-renewable credential (an API key) was rejected. |
| `unauthenticated` | The platform's `UNAUTHENTICATED` status is still recoverable from the error. |
| `unavailable` | The platform's `UNAVAILABLE` status is still recoverable from the error. |
| `failed` | The call did not succeed, for any reason. |

`check` keys: `presented`, `refresh_bearers`, `refreshes`, `refresh_calls` (as
recorded by the server); `session` (`"absent"`, or an object whose fields must
match the stored file, with `"expires": "future"` meaning `expires_at` is after
now); `session_mode` and `dir_mode` (octal permission bits, skipped where the
platform has no POSIX modes); `stray_files` (entries in the session's directory
other than the session file).

### `requires`

| Value | Skip the case when |
|---|---|
| `unwritable_directory` | The harness runs with privileges that ignore directory permissions (for example as root). |
