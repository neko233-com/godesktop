# Architecture contracts

- UI thread owns mutable view/document state. Background work receives immutable
  snapshots and returns through Context.Dispatch with version/identity checks.
- Windows uses Win32, D3D12/DXIL/DirectWrite; macOS uses AppKit/Metal/CoreText.
  Fence/completion ownership governs three-slot buffers and glyph resources.
- The editable document API uses UTF-16 protocol positions with UTF-8 storage.
  Immutable snapshots prepare cancellable transactions on workers. UI bulk edits
  preflight original buffer identity/version/selection before adopting prepared
  lines; saved revision and existing bounded undo/redo history are retained.
  HistorySnapshot copies only the top undo/redo values and immutable source;
  workers prepare real stack transitions. Commit revalidates identity/version/
  selection/revision/stack metadata, then adopts lines without copying source.
  File-backed large-document browsing must avoid loading/duplicating the full file.
  Buffer.Reload adopts a saved disk revision with monotonic versions, immutable
  snapshots and bounded undo history, including LF/CRLF transitions. The caller
  decides whether local edits may be discarded; reloading does not authorize it.
- LSP and VSIX transports are bounded, cancellable and independently terminated.
  Critical native edits use acknowledged RPC rather than droppable notifications.
- gocode consumes published framework versions, remains a public independent
  repository, and is pinned by a reproducible parent gitlink.
- Its bounded native split tree shares canonical document/history/service
  ownership while keeping per-group selection/scroll/tabs and large-file page
  state. Shared reader/index lifetime extends beyond the original view; close
  prompts cover unique dirty resources and late open/captured clicks retain
  group identity. Exact scope and remaining API/layout gaps are in child groups.md.
- Its native startup opens documents and scans Explorer on bounded workers,
  transferring private buffers/indexes to the UI once. Navigation/closed-path
  tickets preserve newer focus and unsaved identity. Commands/providers await
  acknowledged Node document notifications; initialization alone is insufficient.
- Installation, updates, sidecars and user settings must have separate lifecycles.
  An update must preserve workspaces/settings and never silently trust a mirror's
  replacement executable without checking publisher-controlled metadata.
- The native workbench remains functional without Electron. Optional webview
  extension content must not become the application's UI shell.
- gocode content search owns one cancellable worker/latest request and publishes
  immutable editor snapshots. Streaming chunks/regex candidates, bounded ignore
  traversal and virtual visible rows avoid whole large-file or result-tree copies.
  Navigation verifies matched bytes/UTF-16 coordinates, document version/instance,
  focus and query generation before native selection or file-backed byte movement.
- Replacement prepares complete immutable sources off UI, validates all disk and
  buffer states, runs open hooks before final preflight, then commits all buffers
  without interleaved callbacks. Per-file background saves follow, retaining dirty
  undoable text on late conflicts. Captured click identity protects the reviewed
  plan. Bounded metadata groups use a separate worker to prepare genuine global
  Undo/Redo, then preflight and commit all buffers before callbacks. This does not
  provide filesystem-wide atomicity or history across closed resources.
- Immutable Go bitmaps transfer premultiplied RGBA copies to a UI-owned native
  128-entry / 64 MiB cache. Current-frame assets are pinned before any eviction;
  completed GPU frames own texture/staging lifetime. Device recovery retains
  native CPU assets and uploads to fresh device resources; Run shutdown clears
  residency. Glyph R8/RGBA resources and user bitmap RGBA accounting remain
  separate; color-glyphs.md records intrinsic RGB, batching and actual byte bounds.
- Viewport offsets are application state; layout clamps them to content extents
  and uses one inherited clip for GPU commands and hit/focus targets. All visible
  explicit keys expose clipped geometry; passive keys do not enter focus order.
  Scroll deltas stay separate from client pointer coordinates. Key release/focus
  cancellation ends held modifier gestures without retaining closed documents.
