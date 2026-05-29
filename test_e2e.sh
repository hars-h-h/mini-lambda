#!/bin/bash
set -e

AUTH="http://localhost:8081"
FAAS="http://localhost:8080"

echo "=== 1. Register user ==="
curl -s -X POST "$AUTH/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"dev@faas.dev","password":"secret123"}'
echo

echo "=== 2. Login ==="
TOKEN=$(curl -s -X POST "$AUTH/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"dev@faas.dev","password":"secret123"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
echo "Token: $TOKEN"
echo

echo "=== 3. Register function (with token) ==="
CODE='def handler(event):
    name = event.get("name", "world")
    return f"Hello, {name}!"'

curl -s -X POST "$FAAS/register" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary "{\"name\":\"hello\",\"code\":$(echo "$CODE" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')}"
echo

echo "=== 4. Invoke WITHOUT token (expect 401) ==="
curl -s -X POST "$FAAS/invoke/hello" \
  -H "Content-Type: application/json" \
  -d '{"payload":{"name":"FaaS"}}'
echo

echo "=== 5. Invoke WITH token ==="
curl -s -X POST "$FAAS/invoke/hello" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"payload":{"name":"FaaS"}}'
echo

echo "=== 6. Stats ==="
curl -s "$FAAS/stats" -H "Authorization: Bearer $TOKEN"
echo

echo "Done."
