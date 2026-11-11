#!/usr/bin/env bash
set -euo pipefail
release=${1:?release obrigatório}
base=/opt/meu-financeiro
test -x "$release/backend/server"
test -s "$release/frontend/dist/index.html"
ln -s "$release" "$release/current.next"
mv -Tf "$release/current.next" "$base/current"
sudo -n /usr/bin/systemctl restart meu-financeiro.service
for attempt in {1..15}; do
  if curl -fsS --max-time 3 http://127.0.0.1:8080/api/health; then exit 0; fi
  sleep 1
done
echo "O serviço falhou na verificação de saúde. Consulte journalctl -u meu-financeiro." >&2
exit 1
