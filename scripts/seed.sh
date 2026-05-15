#!/bin/bash
# seed.sh — crea y autoriza órdenes de ejemplo
# uso: bash scripts/seed.sh [URL]

BASE="${1:-http://localhost:8080}"

orders=(
  '{"tracking_key":"TRK-2024-001","amount":15000,"destination":"DEST-001122334455","priority":9}'
  '{"tracking_key":"TRK-2024-002","amount":5000,"destination":"DEST-998877665544","priority":5}'
  '{"tracking_key":"TRK-2024-003","amount":250000,"destination":"DEST-112233445566","priority":10}'
  '{"tracking_key":"TRK-2024-004","amount":1200,"destination":"DEST-556677889900","priority":3}'
  '{"tracking_key":"TRK-2024-005","amount":99999,"destination":"DEST-221100998877","priority":7}'
)

echo "Creando órdenes en $BASE..."
ids=()

for body in "${orders[@]}"; do
  resp=$(curl -sf -X POST "$BASE/orders" \
    -H "Content-Type: application/json" \
    -d "$body")
  if [ $? -eq 0 ]; then
    id=$(echo "$resp" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    ids+=("$id")
    key=$(echo "$body" | grep -o '"tracking_key":"[^"]*"' | cut -d'"' -f4)
    echo "  ✓ $key → ${id:0:8}..."
  else
    echo "  ✗ error creando orden"
  fi
done

echo ""
echo "Autorizando órdenes..."
for id in "${ids[@]}"; do
  resp=$(curl -sf -X PUT "$BASE/orders/$id/authorize")
  if [ $? -eq 0 ]; then
    echo "  ✓ ${id:0:8}... autorizada"
  else
    echo "  ✗ error autorizando ${id:0:8}..."
  fi
done

echo ""
echo "Listo. El worker procesará las órdenes en los próximos segundos."
echo "Ejecuta: curl $BASE/orders | python3 -m json.tool"
