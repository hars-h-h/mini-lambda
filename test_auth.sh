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

echo "=== 3. Register a function (with token) ==="
curl -s -X POST "$FAAS/register" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"hello","code":"def handler(event):\n    return f\"Hello, {event.get(chr(110)+chr(97)+chr(109)+chr(101), chr(119)+chr(111)+chr(114)+chr(108)+chr(100))}!\""}'
echo

echo "=== 4. Invoke without token (expect 401) ==="
curl -s -X POST "$FAAS/invoke/hello" \
  -H "Content-Type: application/json" \
  -d '{"payload":{"name":"FaaS"}}'
echo

echo "=== 5. Invoke with token ==="
curl -s -X POST "$FAAS/invoke/hello" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"payload":{"name":"FaaS"}}'
echo

echo "=== 6. Stats ==="
curl -s "$FAAS/stats" -H "Authorization: Bearer $TOKEN"
echo

echo "Done."
