#!/bin/sh
set -eu

: "${MEMENTO_URL:?set MEMENTO_URL, for example http://memento.local:18081}"
: "${1:?usage: upload-with-stage-ticket.sh ZIP UPLOAD_TICKET}"
: "${2:?missing upload ticket}"

zip=$1
ticket=$2

curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/zip' \
  --header "X-Memento-Upload-Ticket: ${ticket}" \
  --data-binary "@${zip}" \
  "${MEMENTO_URL%/}/assets/staging/upload"
