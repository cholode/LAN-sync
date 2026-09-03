# Messages service

- `api`: history/search HTTP surface and message repositories.
- `producer`: accepted-message Kafka producer and live Redis notification.
- `archiver`: Kafka consumer, durable write and recent-message cache.
- `search`: Elasticsearch adapter.

These packages are kept in one Go module during migration. The intended next
step is to add separate `cmd/api`, `cmd/processor`, `cmd/dispatcher` and
`cmd/indexer` entry points without moving their implementations again.

Kafka events are cross-service contracts and therefore live in
`contracts/events`, not inside this service.
