#!/bin/bash
set -euo pipefail

BOOTSTRAP_SERVER="${KAFKA_BOOTSTRAP_SERVER:-kafka:29092}"
KAFKA_BIN="/opt/kafka/bin/kafka-topics.sh"
MAX_RETRIES=30

wait_for_kafka() {
  echo "Waiting for Kafka broker at ${BOOTSTRAP_SERVER}..."
  for i in $(seq 1 "$MAX_RETRIES"); do
    if $KAFKA_BIN --bootstrap-server "$BOOTSTRAP_SERVER" --list >/dev/null 2>&1; then
      echo "Kafka broker is ready."
      return 0
    fi
    echo "  attempt ${i}/${MAX_RETRIES}..."
    sleep 2
  done
  echo "Kafka broker not ready after ${MAX_RETRIES} attempts."
  exit 1
}

create_topic() {
  local topic="$1"
  local partitions="$2"
  local config="${3:-}"

  for i in $(seq 1 5); do
    if $KAFKA_BIN --bootstrap-server "$BOOTSTRAP_SERVER" --list 2>/dev/null | grep -qx "$topic"; then
      echo "Topic already exists: $topic"
      return 0
    fi

    echo "Creating topic: $topic (partitions=$partitions, attempt=$i)"
    if [ -n "$config" ]; then
      if $KAFKA_BIN --bootstrap-server "$BOOTSTRAP_SERVER" \
        --create --if-not-exists \
        --topic "$topic" \
        --partitions "$partitions" \
        --replication-factor 1 \
        --config "$config"; then
        return 0
      fi
    else
      if $KAFKA_BIN --bootstrap-server "$BOOTSTRAP_SERVER" \
        --create --if-not-exists \
        --topic "$topic" \
        --partitions "$partitions" \
        --replication-factor 1; then
        return 0
      fi
    fi
    sleep 3
  done

  echo "Failed to create topic: $topic"
  exit 1
}

wait_for_kafka

# 架构文档定义的核心 Topic
create_topic "game.round.settled" 6 "compression.type=snappy"
create_topic "game.event.bigwin" 3 "compression.type=snappy"
create_topic "game.wallet.rollback" 3 "compression.type=snappy"

echo "Kafka topics initialized:"
$KAFKA_BIN --bootstrap-server "$BOOTSTRAP_SERVER" --list
