# Gateway boundary

This package is the HTTP composition layer. It owns routes, middleware and HTTP
server policy, but not message persistence or file processing.

To extract it into a container later, add a `cmd/gateway` composition root and
replace the in-process `messages` and `files` route registrations with clients.
The public paths remain unchanged, so Nginx and frontend clients do not change.
