#!/bin/sh
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"
if [ ! -x backend/server ] || [ ! -f frontend/dist/index.html ]; then
  echo 'Execute make install e make build antes de iniciar a demonstração.' >&2
  exit 1
fi
cd backend
export MF_DATABASE=../data/demo.db
export MF_ADDR=127.0.0.1:8080
export MF_ORIGIN=http://127.0.0.1:8080
export MF_FRONTEND=../frontend/dist
exec ./server --demo
