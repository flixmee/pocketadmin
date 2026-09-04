Goal (incl. success criteria):

- Fix the reported build errors caused by incompatible ozzo-validation import paths.
- Success means the codebase uses one validation module path and the affected packages build/tests pass as far as unrelated merge conflicts allow.

Constraints/Assumptions:

- Follow `AGENTS.md`; preserve unrelated user changes and unresolved conflicts.
- The repository standard is `github.com/pocketbase/ozzo-validation/v4`, which is already used throughout the existing code and required by `go.mod`.

Key decisions:

- Replace all `github.com/go-ozzo/ozzo-validation/v4` imports in the new Go files with `github.com/pocketbase/ozzo-validation/v4`; remove the now-unused duplicate module requirement/checksums.

State:
  - Done:
    - Inspected both merge stages and the associated base options.
    - Resolved `core/collection_model.go` by retaining `collectionBaseOptions` and applying `json.Deterministic(true)` to the merged value.
    - Confirmed the file parses/formats cleanly, contains no conflict markers, and passes `git diff --check`.
    - Staged `core/collection_model.go`, so Git now considers that file resolved.
    - Found 19 Go files importing the incompatible upstream ozzo module while the rest of the repository uses the PocketBase fork.
    - Replaced all 19 incompatible imports with `github.com/pocketbase/ozzo-validation/v4`.
    - Removed the unused `github.com/go-ozzo/ozzo-validation/v4` requirement and checksums.
    - Confirmed no incompatible imports remain and `git diff --check` passes.
    - `GOCACHE=/tmp/pocketadmin-go-cache go build ./core` passed.
    - `GOCACHE=/tmp/pocketadmin-go-cache go build ./...` passed; Go emitted only a non-fatal module stat-cache permission warning outside the writable workspace.
  - Now:
    - Build fix is complete.
  - Next:
    - Resolve the separate test/UI merge conflicts before running the full test suite.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `core/collection_model.go`
- Go files currently importing `github.com/go-ozzo/ozzo-validation/v4`
- `go.mod`
- `go.sum`
- `GOCACHE=/tmp/pocketadmin-go-cache go build ./core`
- `GOCACHE=/tmp/pocketadmin-go-cache go build ./...`
