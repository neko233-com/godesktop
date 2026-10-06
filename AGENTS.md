# godesktop engineering harness

Maintain `agent docs/` as the engineering record for this repository and the
public gocode submodule. Read its README and current status before continuing.

- Keep the UI native Go + platform GPU APIs. Do not replace the workbench with
  Electron or a browser shell. Node is an optional isolated extension/AI runtime.
- Treat user instructions and the active objective as authoritative. Repository
  guidance does not add an approval gate to already authorized work.
- Capture source revisions, accepted behavior, limitations and real validation
  evidence. A roadmap item or a mock response is not proof of implementation.
- Preserve immutable snapshots and UTF-16 protocol coordinates. Dispatch native
  state changes to the UI thread; keep file/network/process work off that thread.
- Use bounded memory/queues and explicit shutdown for large files, render slots,
  language servers, extensions and update helpers.
- Test the published module independently with GOWORK=off before advancing the
  submodule gitlink. Do not move published tags.
- Run workflow linting for CI/packaging changes, appropriate race/strict-cgo and
  native acceptance, and git diff --check. Record exact CI source commits.
- Maintain upstream source/license provenance for copied assets. Keep credentials,
  signing keys, caches, generated installers and user workspaces out of Git.
- Use free distribution tooling. Do not buy certificates, notarization services
  or installer tooling to satisfy the current objective.

Update the relevant `agent docs/` documents with each functional milestone.
