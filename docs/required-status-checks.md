# Required status checks (Phase 8)

Required status checks are a repository setting (Settings > Branches >
branch protection rule > "Require status checks to pass before merging"),
not something expressible in a workflow YAML file. This doc exists so that
setting isn't left undiscoverable — it lists the exact check names GitHub
will show once each workflow has run at least once on a PR against `main`.

## Why this is a doc and not a file the workflows manage

GitHub's branch-protection API can be scripted, but doing so from inside
these same workflows would mean a workflow modifying the protection rule
that gates its own merge — a circular dependency, and a change to
repository security settings that shouldn't happen silently as a side
effect of CI. Configure this once, by hand, in Settings.

## Checks to require on `main` for `services/auth-service/**`

For a `uses:` call to a reusable workflow, GitHub reports each inner job as
`<caller job name> / <called job name>`. Based on the current
`auth-service-ci.yml` wiring:

| Check name | Source | Blocks merge if |
|---|---|---|
| `lint-test / lint` | reusable-go-lint-test.yml | gofmt or go vet fails |
| `lint-test / test` or `lint-test / test-with-db` | reusable-go-lint-test.yml | tests fail, or coverage < threshold (only one of these two runs, depending on `needs-db`) |
| `security-gates / codeql` | reusable-security-gates.yml | CodeQL finds a blocking alert |
| `security-gates / govulncheck` | reusable-security-gates.yml | a known-vulnerable dependency is in use |
| `security-gates / license-check` | reusable-security-gates.yml | a dependency license isn't in the allowed list |
| `build / build` | reusable-docker-build-publish.yml | image build fails, or Trivy finds a CRITICAL/HIGH vulnerability |
| `gitleaks` | secret-scan.yml (standalone, not a `uses:` call, so no parent-job prefix) | a secret is detected anywhere in the diff or history |

## Checks to require on Terraform stacks (once a caller workflow exists)

| Check name | Source | Blocks merge if |
|---|---|---|
| `<caller job> / fmt-validate` | reusable-terraform-ci.yml | `terraform fmt` or `terraform validate` fails |
| `<caller job> / plan` | reusable-terraform-ci.yml | `terraform plan` errors out |

`apply` is intentionally not in this list — it only runs post-merge on
push to `main`, gated by the required GitHub Environment reviewers, not by
PR status checks.

## Before enabling

Check names only appear in the branch-protection picker after they've run
at least once on a PR. Open (or re-run) one PR that touches
`services/auth-service/**` first, then come back to Settings > Branches
and select the names above from the list — don't hand-type them, since a
typo silently means the check never blocks anything.
