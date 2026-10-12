#!/bin/bash
set -euo pipefail
[[ "${POC_STREAM}" =~ ^[0-9a-f]{32}$ ]]
topic="poc.spire.commands.${POC_STREAM}"
group="poc.spire.money.${POC_STREAM}"
client=/config/admin.properties
/opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9093 --command-config "$client" --create --if-not-exists --topic "$topic" --partitions 1 --replication-factor 1 --config max.message.bytes=65536
for principal in kafka-producer kafka-unidentified; do
 /opt/kafka/bin/kafka-acls.sh --bootstrap-server kafka:9093 --command-config "$client" --add --allow-principal "User:CN=$principal" --operation Write --operation Describe --topic "$topic"
done
/opt/kafka/bin/kafka-acls.sh --bootstrap-server kafka:9093 --command-config "$client" --add --allow-principal User:CN=kafka-money --operation Read --operation Describe --topic "$topic"
/opt/kafka/bin/kafka-acls.sh --bootstrap-server kafka:9093 --command-config "$client" --add --allow-principal User:CN=kafka-money --operation Read --group "$group"
