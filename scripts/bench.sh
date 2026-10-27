
#!/usr/bin/env bash

set -euo pipefail

url="${1:-http://localhost:8080/}"

echo "Sequential:"
time (
    seq 1 100 | xargs -P 1 -I '{}' \
        curl --fail --silent --output /dev/null "$url"
)

echo "Concurrent:"
time (
    seq 1 100 | xargs -P 100 -I '{}' \
        curl --fail --silent --output /dev/null "$url"
)
