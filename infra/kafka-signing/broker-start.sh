#!/bin/bash
set -euo pipefail
if [ ! -f /var/lib/kafka/data/meta.properties ]; then
  /opt/kafka/bin/kafka-storage.sh format --config /config/kafka.properties --cluster-id "$(cat /config/cluster-id)"
fi
exec /opt/kafka/bin/kafka-server-start.sh /config/kafka.properties
