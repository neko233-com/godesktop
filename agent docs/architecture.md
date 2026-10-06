# Architecture contracts

- UI thread owns mutable view/document state. Background work receives immutable
  snapshots and returns through Context.Dispatch with version/identity checks.
- Windows uses Win32, D3D12/DXIL/DirectWrite; macOS uses AppKit/Metal/CoreText.
  Fence/completion ownership governs three-slot buffers and glyph resources.
- The editable document API uses UTF-16 protocol positions with UTF-8 storage.
  File-backed large-document browsing must avoid loading/duplicating the full file.
- LSP and VSIX transports are bounded, cancellable and independently terminated.
  Critical native edits use acknowledged RPC rather than droppable notifications.
- gocode consumes published framework versions, remains a public independent
  repository, and is pinned by a reproducible parent gitlink.
- Installation, updates, sidecars and user settings must have separate lifecycles.
  An update must preserve workspaces/settings and never silently trust a mirror's
  replacement executable without checking publisher-controlled metadata.
- The native workbench remains functional without Electron. Optional webview
  extension content must not become the application's UI shell.
