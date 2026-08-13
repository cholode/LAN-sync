#!/bin/sh
set -e
# File storage is handled by MinIO / OSS; local uploads/temp_chunks directories are no longer needed.
if [ "$(id -u)" = "0" ]; then
	exec su-exec imuser:nobody "$@"
fi
exec "$@"
