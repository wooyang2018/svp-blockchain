#!/bin/bash

# Find all containers created from the chain-proxy image
container_ids=$(docker ps -a -q --filter "ancestor=chain-proxy")

# Check if there are any chain-proxy containers
if [ -z "$container_ids" ]; then
  echo "No containers found created from the chain-proxy image."
  exit 0
fi

# Stop all chain-proxy containers
echo "=== Stopping containers created from the chain-proxy image ==="
for container_id in $container_ids; do
  docker stop "$container_id" > /dev/null
  echo "Container $container_id stopped"
done

# Remove all chain-proxy containers
echo "=== Removing containers created from the chain-proxy image ==="
for container_id in $container_ids; do
  docker rm "$container_id" > /dev/null
  echo "Container $container_id removed"
done
