#!/bin/bash

# Check if the number of containers is passed as a parameter
if [ -z "$1" ]; then
  echo "Usage: $0 <container_count>"
  exit 1
fi

echo "=== Initial system resource usage ==="
echo "CPU usage:"
mpstat 1 1

echo "Memory usage:"
free -h

# Start the specified number of Docker containers
container_count=$1
container_ids=()

for ((i=1; i<=container_count; i++)); do
  port=$((8080 + i))
  container_id=$(docker run -d -p ${port}:8080 chain-proxy)
  container_ids+=("$container_id")
  echo "Container started on port ${port}, container ID: ${container_id}"
done

sleep 10

echo "=== System resource usage after 10 seconds ==="
echo "CPU usage:"
mpstat 1 1

echo "Memory usage:"
free -h

# Send /setup/oneclick requests to each container
echo "=== Sending /setup/oneclick requests to each container ==="
for ((i=1; i<=container_count; i++)); do
  port=$((8080 + i))
  echo "Sending request to container on port ${port}..."

  # Send POST request to /setup/oneclick
  curl -X POST http://localhost:${port}/setup/oneclick \
  -H "Content-Type: application/json" \
  -d '{
    "nodeCount": 4,
    "stakeQuota": 9999,
    "windowSize": 4
  }'
done

sleep 60

# Check the /proxy/-1/consensus status
for ((i=1; i<=container_count; i++)); do
  port=$((8080 + i))
  echo "Checking status for container on port ${port}..."
  response_code=$(curl -o /dev/null -s -w "%{http_code}" http://localhost:${port}/proxy/-1/consensus)

  if [ "$response_code" -eq 200 ]; then
    echo "Container on port ${port} started successfully"
  else
    echo "Container on port ${port} failed to start"
  fi
done

# Print CPU and memory usage again
echo "=== System resource usage after requests ==="
echo "CPU usage:"
mpstat 1 1

echo "Memory usage:"
free -h
