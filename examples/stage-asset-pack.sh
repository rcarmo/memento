#!/bin/sh
set -eu

: "${MEMENTO_URL:?set MEMENTO_URL, for example http://memento.local:18081}"
: "${MEMENTO_TOKEN:?set MEMENTO_TOKEN without printing it}"
: "${1:?usage: stage-asset-pack.sh ZIP ASSET_KIND VERSION IDEMPOTENCY_KEY}"
: "${2:?missing asset kind}"
: "${3:?missing version}"
: "${4:?missing idempotency key}"

zip=$1
kind=$2
version=$3
key=$4

curl --fail-with-body \
  --request POST \
  --header "Authorization: Bearer ${MEMENTO_TOKEN}" \
  --header 'Content-Type: application/zip' \
  --header "Idempotency-Key: ${key}" \
  --header "X-Memento-Asset-Kind: ${kind}" \
  --header "X-Memento-Asset-Version: ${version}" \
  --data-binary "@${zip}" \
  "${MEMENTO_URL%/}/assets/staging"
