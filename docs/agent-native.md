# Agent-Native Kubernetes: Why DeployScope Speaks to Machines

## The Problem

An autonomous agent gets access to a Kubernetes cluster. It has `kubectl`. It can list pods, describe deployments, read events. It has *data*. It does not have *understanding*.

```bash
kubectl get deployments --all-namespaces
```

This returns 200 lines of text. The agent now knows names and replica counts. It does not know:

- Who owns each service
- Which services are critical vs best-effort
- Where the GitOps repo is for any of them
- Who to escalate to when something is red
- Whether the service that just went red has been red for 3 hours or 3 seconds
- What depends on the broken service
- Whether to page someone or just log it

The gap between *data* and *actionable understanding* is where most autonomous infrastructure pipelines break down. An agent that can only report "auth-service has 0/3 ready replicas" creates work for humans. An agent that can report "auth-service is a critical-tier service, red for 3 hours, owned by team-platform, oncall at #platform-oncall, GitOps repo at github.com/org/infra path clusters/prod/auth/, depends on postgres-platform which is healthy — suggest rollback" creates a *pull request*.

## The Idea

DeployScope is a mirror — it reads Kubernetes state and presents it. Today it serves a web dashboard and REST API for humans. The next step is making it speak to machines just as fluently.

One binary. One command. Full situational awareness.

```bash
deployscope status --format json
```

The output tells an agent everything it needs to proceed: what's running, what's healthy, who owns it, where the code lives, and what to do next. Not because DeployScope invents this knowledge — but because teams declare it through annotations on their deployments, and DeployScope surfaces it.

## How It Works

### Integration Pointers

Kubernetes already has a label convention (`app.kubernetes.io/*`). DeployScope extends this with annotations that carry the context agents need:

```yaml
metadata:
  annotations:
    deployscope.dev/owner: "team-platform"
    deployscope.dev/oncall: "#platform-oncall"
    deployscope.dev/tier: "critical"
    deployscope.dev/gitops-repo: "github.com/org/infra"
    deployscope.dev/gitops-path: "clusters/prod/auth/"
    deployscope.dev/runbook: "https://wiki.internal/auth-runbook"
    deployscope.dev/dashboard: "https://grafana.internal/d/auth"
    deployscope.dev/depends-on: "postgres-platform,redis-shared"
```

Services that don't have annotations still appear — with null fields, not omitted keys. The schema is stable. Gaps are visible. Adoption is gradual.

### Opt-Out

Not everything should be in the agent's world view. Test fixtures, infra internals, canary deployments — these are noise for an autonomous pipeline.

```yaml
metadata:
  annotations:
    deployscope.dev/ignore: "true"
```

Ignored deployments are invisible: not in status output, not in counts, not in the summary. They don't exist from the agent's perspective.

### Deterministic Routing

DeployScope doesn't ask an LLM what to do. It applies rules:

| Tier | Status | Action |
|------|--------|--------|
| critical | red | escalate — page oncall, P0 |
| critical | yellow | warn — notify oncall, P1 |
| standard | red | inform — create WO, P1 |
| standard | yellow | log — create WO, P2 |
| best-effort | red | log — create WO, P2 |
| best-effort | yellow | observe — no action |

The agent gets a `routing` section in the JSON output that tells it exactly what to do. No judgment required. No hallucination possible.

### Agent Readiness Score

Not every cluster is ready for autonomous agents. If only 30% of deployments have owner annotations, an agent will create vague WOs for the other 70%.

`deployscope doctor` reports an **agent-readiness score** — the percentage of deployments with enough annotations for an agent to create actionable work orders. This drives adoption: teams see "your cluster is 38% agent-ready" and know what to fix.

## The Full Pipeline

In a fully autonomous infrastructure, the chain looks like this:

```
Discovery agent
    │
    ├─ reads SKILL.md (ANCC convention — knows what deployscope can do)
    ├─ runs: deployscope doctor (pre-flight — can I reach K8s? do I have RBAC?)
    └─ runs: deployscope status --format json
         │
         ├─ cluster: prod-us-east-1, 47 services, 3 degraded
         ├─ auth-service: critical, red 3h, postgres-platform healthy (not upstream)
         ├─ routing: escalate to #platform-oncall, P0
         └─ gitops: github.com/org/infra, path clusters/prod/auth/
              │
              v
         Creates work order with full context
              │
              v
GitOps agent picks up WO
    │
    ├─ clones github.com/org/infra
    ├─ reads clusters/prod/auth/
    ├─ creates revert PR (v1.5.0 → v1.4.2)
    └─ PR assigned to team-platform for review
```

Every step is deterministic. Every execution boundary is enforced by [chainwatch](https://github.com/ppiankov/chainwatch). Every file read is redacted by [pastewatch](https://github.com/ppiankov/pastewatch). No human in the loop until the PR review.

## The Ecosystem

DeployScope doesn't work alone. It occupies a specific position in the stack:

| Layer | Tool | What It Does |
|-------|------|-------------|
| **Cognitive** | DeployScope | Reads K8s state + annotations, tells agents what exists, what's healthy, who owns it, where to go |
| **Enforcement** | [Chainwatch](https://github.com/ppiankov/chainwatch) | Runtime control plane — allows/denies agent actions at execution boundaries |
| **Redaction** | [Pastewatch](https://github.com/ppiankov/pastewatch) | Secret detection — ensures sensitive data never leaves the machine |
| **Investigation** | [Nullbot](https://github.com/ppiankov/chainwatch) | LLM-driven observer under chainwatch enforcement — generates structured findings |

DeployScope is the **first contact** — the tool an agent uses before it does anything else. It answers: "What is this cluster? Is it healthy? Who owns what? Where do I go to fix things?"

## Design Principles

**Mirror, not oracle.** DeployScope reads annotations that teams declare. It never invents data. If a service has no owner annotation, the field is null — not guessed.

**Gaps are visible, not hidden.** Missing annotations show as null fields, not omitted keys. The schema is always the same shape. Agents don't need special handling for incomplete data.

**Deterministic routing, not AI advice.** The routing rules are hardcoded: tier + status = action. An agent cannot hallucinate a different priority. The system decides severity, not the model.

**Opt-out is structural.** `deployscope.dev/ignore` doesn't just filter — it makes deployments invisible. There's no way for an agent to "discover" an ignored service.

**No central database.** Source of truth stays in Kubernetes manifests. No CMDB to sync, no config database to drift. Teams annotate their own deployments, and the data is always fresh.

**Safe by default.** The `--redact` flag scrubs potentially sensitive values from annotations before output. Safe to pipe to cloud LLMs without leaking internal URLs or tokens.

## What DeployScope Is NOT

This hasn't changed. DeployScope does not:

- **Mutate** anything — read-only RBAC, no controllers, no operators
- **Store history** — temporal context comes from K8s conditions, not a database
- **Run ML** — all routing is deterministic rules, not classification
- **Replace a CMDB** — it mirrors what's declared, nothing more
- **Auto-remediate** — it tells agents what's wrong, not how to fix it (that's the GitOps agent's job, under chainwatch enforcement)

## Getting Started with Annotations

Run `deployscope init` to generate:
1. A `deployscope.yaml` config file with cluster identity
2. An example annotation YAML that teams can apply to their deployments

Start with `owner` and `tier` — those two annotations alone let agents distinguish "page someone" from "log it." Add `gitops-repo` and `gitops-path` when you're ready for the full autonomous pipeline.
