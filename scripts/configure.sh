#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
if [ -e .env ]; then
  echo 'O arquivo .env já existe e foi preservado.'
  exit 0
fi
umask 077
set -C
cat > .env <<EOF
POSTGRES_USER=oficina
POSTGRES_DB=oficina
POSTGRES_PASSWORD=$(openssl rand -hex 24)
ADMIN_USERNAME=admin
ADMIN_PASSWORD=$(openssl rand -hex 12)
JWT_SECRET=$(openssl rand -hex 32)
DEMO_DATA=true
API_PORT=8080
DB_PORT=5432
EOF
echo 'Arquivo .env criado. Consulte ADMIN_USERNAME e ADMIN_PASSWORD nele para autenticar na API.'
