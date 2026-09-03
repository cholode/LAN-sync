# Messages boundary

This package owns message persistence (MySQL/MongoDB), history queries and
search endpoints. `RegisterRoutes` is the current in-process adapter.

For a standalone service, reuse the repositories and handlers behind a new
`cmd/messages` entrypoint. Authentication/room membership should arrive as a
verified identity or be checked through an authorization service.
