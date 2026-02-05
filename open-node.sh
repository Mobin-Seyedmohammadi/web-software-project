#!/usr/bin/env sh

# Detect OS for Docker networking
if [ "$(uname)" = "Linux" ]; then
    NETWORK="--network host"
else
    # macOS/Windows: use host.docker.internal to access host
    NETWORK="--add-host=host.docker.internal:host-gateway"
fi

docker run -it --rm -v "$(pwd):/src" -u "$(id -u):$(id -g)" -e HOME=/tmp -p 5173:5173 -p 4173:4173 $NETWORK --workdir /src/webui node:20 /bin/bash
