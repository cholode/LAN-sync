# Files boundary

This package owns presigned uploads, completion metadata and downloads.
Object storage is injected through `storage.Provider`; route registration is
isolated from the gateway.

For a standalone service, create a `cmd/files` entrypoint and construct the
module with its own storage and metadata dependencies. Existing URLs can be
proxied by the gateway without changing clients.
