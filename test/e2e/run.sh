#!/usr/bin/env bash

# Copyright 2026 The HAMi Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

diagnostics() {
  kubectl get pods,resourceclaims -A -o wide || true
  kubectl get resourceslices,deviceclasses -o yaml || true
  kubectl logs -n hami-dra-e2e daemonset/fake-drivers --all-containers --tail=-1 || true
  kubectl logs -n hami-dra-system deployment/hami-dra --tail=-1 || true
}
trap diagnostics ERR

kubectl apply -f "${repo_root}/test/e2e/fake-multi-vendor.yaml"
kubectl rollout status -n hami-dra-e2e daemonset/fake-drivers --timeout=120s

for driver in hami-core-gpu.project-hami.io dra.hygon.com ascend.project-hami.io; do
  for _ in $(seq 1 60); do
    slice=$(kubectl get resourceslices -o jsonpath="{.items[?(@.spec.driver=='${driver}')].metadata.name}" | awk '{print $1}')
    test -n "${slice}" && break
    sleep 2
  done
  test -n "${slice}"
done

helm upgrade --install hami-dra "${repo_root}/charts/hami-dra" \
  --namespace hami-dra-system --create-namespace \
  --set 'deviceVendors={nvidia,hygon,ascend}' \
  --set drivers.nvidia.enabled=false \
  --set drivers.fake.enabled=false \
  --set monitor.enabled=false \
  --set webhook.image.registry='' \
  --set webhook.image.repository=hami-dra-webhook \
  --set webhook.image.tag=e2e \
  --set webhook.image.pullPolicy=Never \
  --wait --timeout=180s

kubectl apply -f "${repo_root}/test/e2e/multi-vendor-pod.yaml"
kubectl wait -n hami-dra-workloads pod/multi-vendor --for=condition=Ready --timeout=180s
kubectl wait -n hami-dra-workloads pod/no-device --for=condition=Ready --timeout=180s
test -z "$(kubectl get pod -n hami-dra-workloads no-device -o jsonpath='{.spec.resourceClaims[*].resourceClaimName}')"
test -z "$(kubectl get pod -n hami-dra-workloads no-device -o jsonpath='{.metadata.labels.hami\.io/dra}')"

claims=$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.spec.resourceClaims[*].resourceClaimName}')
test "$(wc -w <<<"${claims}" | tr -d ' ')" = 3
for suffix in nvidia hygon ascend310p; do
  grep -q -- "-${suffix}" <<<"${claims}"
done

test -z "$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.spec.containers[0].resources.limits.nvidia\.com/gpu}')"
test -z "$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.spec.containers[0].resources.limits.hygon\.com/hcunum}')"
test -z "$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.spec.containers[0].resources.limits.huawei\.com/Ascend310P}')"

for claim in ${claims}; do
  kubectl wait -n hami-dra-workloads "resourceclaim/${claim}" --for=jsonpath='{.status.allocation}' --timeout=180s
done

declare -A expected_classes=(
  [nvidia]=hami-core-gpu.project-hami.io
  [hygon]=dra.hygon.com
  [ascend310p]=hami-vnpu-core.project-hami.io
)
declare -A expected_drivers=(
  [nvidia]=hami-core-gpu.project-hami.io
  [hygon]=dra.hygon.com
  [ascend310p]=ascend.project-hami.io
)
for suffix in "${!expected_classes[@]}"; do
  claim=$(tr ' ' '\n' <<<"${claims}" | grep -- "-${suffix}$")
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${claim}" -o jsonpath='{.spec.devices.requests[0].exactly.deviceClassName}')" = "${expected_classes[${suffix}]}"
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${claim}" -o jsonpath='{.status.allocation.devices.results[0].driver}')" = "${expected_drivers[${suffix}]}"
done
