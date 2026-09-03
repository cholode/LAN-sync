# Gateway service

- `websocket`: connection, room and shard state local to one gateway process.
- `handlers`: public HTTP/WebSocket handlers.
- `http`: router composition.
- `grpc`: IM gRPC ingress used by the agent runtime.
- `clients`: outbound clients owned by the gateway.
- `Dockerfile.monolith`: compatibility image; currently starts the root
  launcher until message processing is split into its own executable.

The gateway owns live socket objects only. Durable message history, search and
file storage belong to their respective services.
