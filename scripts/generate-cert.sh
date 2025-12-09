#!/bin/bash

# Generate self-signed certificate for webhook
# This script generates a certificate that can be used for local development

set -e

CERT_DIR=${CERT_DIR:-/tmp/k8s-webhook-server/serving-certs}
SERVICE_NAME=${SERVICE_NAME:-webhook}
NAMESPACE=${NAMESPACE:-default}

mkdir -p ${CERT_DIR}

# Generate private key
openssl genrsa -out ${CERT_DIR}/tls.key 2048

# Generate certificate signing request
openssl req -new -key ${CERT_DIR}/tls.key \
  -out ${CERT_DIR}/tls.csr \
  -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc"

# Generate self-signed certificate
openssl x509 -req -days 365 -in ${CERT_DIR}/tls.csr \
  -signkey ${CERT_DIR}/tls.key \
  -out ${CERT_DIR}/tls.crt \
  -extensions v3_req \
  -extfile <(
    cat <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF
  )

echo "Certificate generated in ${CERT_DIR}"
echo "Files:"
ls -lh ${CERT_DIR}/*.{key,crt}

