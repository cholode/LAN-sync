# Users service migration target

User repository interfaces and their GORM implementation live in `repository/`.
Room and membership repositories belong to `services/rooms/repository`.

Gateway/Admin composition roots construct this repository and inject it through
consumer-owned interfaces. No root repository facade or global user repository
remains. Users is not yet a separately deployed service: a users service
contract and remote adapters are still needed for database ownership isolation.
