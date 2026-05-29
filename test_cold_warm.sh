#!/bin/bash
set -e

AUTH="http://localhost:8081"
FAAS="http://localhost:8080"

echo "=== 1. Setup Auth ==="
# Ensure user exists
curl -s -X POST "$AUTH/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"dev@faas.dev","password":"secret123"}' > /dev/null

# Get Token
TOKEN=$(curl -s -X POST "$AUTH/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"dev@faas.dev","password":"secret123"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    echo "Failed to get token"
    exit 1
fi
echo "Token acquired."
echo

echo "=== 2. Register a new function ==="
FUNC_NAME="perf-test-$RANDOM"
CODE='def handler(event):
    name = event.get("name", "world")
    return f"Hello, {name}!"'

curl -s -X POST "$FAAS/register" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary "{\"name\":\"$FUNC_NAME\",\"code\":$(echo "$CODE" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')}" > /dev/null
echo "Function '$FUNC_NAME' registered."
echo

echo "=== 3. Measuring Cold Start (1st Invocation) ==="
echo "Invoking function for the first time..."
COLD_START=$(curl -s -o /dev/null -w "%{time_total}\n" -X POST "$FAAS/invoke/$FUNC_NAME" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"payload":{"name":"Cold"}}')
echo "Cold Start Time: ${COLD_START}s"
echo

echo "=== 4. Measuring Warm Start (Subsequent Invocations) ==="
for i in {1..3}; do
  echo "Warm invocation $i..."
  WARM_START=$(curl -s -o /dev/null -w "%{time_total}\n" -X POST "$FAAS/invoke/$FUNC_NAME" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"payload":{"name":"Warm"}}')
  echo "Warm Start Time ($i): ${WARM_START}s"
done
echo

echo "Done. The cold start should be significantly higher than the warm starts due to container/runtime initialization."
