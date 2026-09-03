# Files service

`api` currently contains the HTTP module and the MinIO/OSS storage adapters.
It remains mounted by the compatibility backend. A future standalone command
can reuse the same module without relocating the implementation.
