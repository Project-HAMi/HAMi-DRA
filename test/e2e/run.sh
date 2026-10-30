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
  kubectl get events -n hami-dra-workloads --sort-by=.metadata.creationTimestamp || true
  kubectl logs -n hami-dra-e2e daemonset/fake-drivers --all-containers --tail=-1 || true
  kubectl logs -n hami-dra-system deployment/hami-dra --tail=-1 || true
}
trap diagnostics ERR

kubectl apply -f "${repo_root}/test/e2e/fake-multi-vendor.yaml"
kubectl rollout status -n hami-dra-e2e daemonset/fake-drivers --timeout=120s

for driver in hami-core-gpu.project-hami.io dra.hygon.com ascend.project-hami.io gpu.amd.com; do
  for _ in $(seq 1 60); do
    slice=$(kubectl get resourceslices -o jsonpath="{.items[?(@.spec.driver=='${driver}')].metadata.name}" | awk '{print $1}')
    test -n "${slice}" && break
    sleep 2
  done
  test -n "${slice}"
done

helm upgrade --install hami-dra "${repo_root}/charts/hami-dra" \
  --namespace hami-dra-system --create-namespace \
  --set 'deviceVendors={nvidia,hygon,ascend,amd}' \
  --set amdReferenceComputeUnits=32 \
  --set drivers.nvidia.enabled=false \
  --set drivers.amd.enabled=false \
  --set drivers.fake.enabled=false \
  --set monitor.enabled=false \
  --set webhook.image.registry='' \
  --set webhook.image.repository=hami-dra-webhook \
  --set webhook.image.tag=e2e \
  --set webhook.image.pullPolicy=Never \
  --wait --timeout=180s

# Recreate workloads so a reused cluster exercises CREATE admission, not UPDATE.
old_claims=$(kubectl get pod -n hami-dra-workloads \
  multi-vendor no-device amd-compute-exhausted amd-memory-exhausted --ignore-not-found \
  -o jsonpath='{.items[*].spec.resourceClaims[*].resourceClaimName}')
kubectl delete pod -n hami-dra-workloads \
  multi-vendor no-device amd-compute-exhausted amd-memory-exhausted \
  --ignore-not-found --wait=true --timeout=120s
for claim in ${old_claims}; do
  kubectl delete -n hami-dra-workloads "resourceclaim/${claim}" \
    --ignore-not-found --wait=true --timeout=120s
done

kubectl apply -f "${repo_root}/test/e2e/multi-vendor-pod.yaml"
kubectl wait -n hami-dra-workloads pod/multi-vendor --for=condition=Ready --timeout=180s
kubectl wait -n hami-dra-workloads pod/no-device --for=condition=Ready --timeout=180s
test -z "$(kubectl get pod -n hami-dra-workloads no-device -o jsonpath='{.spec.resourceClaims[*].resourceClaimName}')"
test -z "$(kubectl get pod -n hami-dra-workloads no-device -o jsonpath='{.metadata.labels.hami\.io/dra}')"

claims=$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.spec.resourceClaims[*].resourceClaimName}')
test "$(wc -w <<<"${claims}" | tr -d ' ')" = 4
for suffix in nvidia hygon ascend310p amd; do
  grep -q -- "-${suffix}" <<<"${claims}"
done

test -z "$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.spec.containers[0].resources.limits.nvidia\.com/gpu}')"
test -z "$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.spec.containers[0].resources.limits.hygon\.com/hcunum}')"
test -z "$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.spec.containers[0].resources.limits.huawei\.com/Ascend310P}')"

for resource in gpu gpumem gpucores; do
  for field in requests limits; do
    test -z "$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath="{.spec.containers[0].resources.${field}.amd\\.com/${resource}}")"
  done
done

for claim in ${claims}; do
  kubectl wait -n hami-dra-workloads "resourceclaim/${claim}" --for=jsonpath='{.status.allocation}' --timeout=180s
done

declare -A expected_classes=(
  [nvidia]=hami-core-gpu.project-hami.io
  [hygon]=dra.hygon.com
  [ascend310p]=hami-vnpu-core.project-hami.io
  [amd]=gpu.amd.com
)
declare -A expected_drivers=(
  [nvidia]=hami-core-gpu.project-hami.io
  [hygon]=dra.hygon.com
  [ascend310p]=ascend.project-hami.io
  [amd]=gpu.amd.com
)
for suffix in "${!expected_classes[@]}"; do
  claim=$(tr ' ' '\n' <<<"${claims}" | grep -- "-${suffix}$")
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${claim}" -o jsonpath='{.spec.devices.requests[0].exactly.deviceClassName}')" = "${expected_classes[${suffix}]}"
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${claim}" -o jsonpath='{.status.allocation.devices.results[0].driver}')" = "${expected_drivers[${suffix}]}"
done

amd_claim=$(tr ' ' '\n' <<<"${claims}" | grep -- '-amd$')
test "$(kubectl get -n hami-dra-workloads "resourceclaim/${amd_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.count}')" = 1

# AMD core requests are percentages; 50% of the fixture's 32 CUs consumes 16 CUs.
test "$(kubectl get -n hami-dra-workloads "resourceclaim/${amd_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.capacity.requests.computeUnits}')" = 16
test -z "$(kubectl get -n hami-dra-workloads "resourceclaim/${amd_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.capacity.requests.cores}')"
test "$(kubectl get -n hami-dra-workloads "resourceclaim/${amd_claim}" -o jsonpath='{.status.allocation.devices.results[0].consumedCapacity.computeUnits}')" = 16

# Requests and allocation results may serialize the same 2 GiB quantity differently.
for memory in \
  "$(kubectl get -n hami-dra-workloads "resourceclaim/${amd_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.capacity.requests.memory}')" \
  "$(kubectl get -n hami-dra-workloads "resourceclaim/${amd_claim}" -o jsonpath='{.status.allocation.devices.results[0].consumedCapacity.memory}')"
do
  test "${memory}" = 2147483648 || test "${memory}" = 2Gi
done

# The mixed Pod leaves 14 GiB and 16 CUs free. Each request below fits the
# complete fake GPU, but exceeds only one of those remaining capacities.
amd_selectors=$(kubectl get -n hami-dra-workloads "resourceclaim/${amd_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.selectors}')
for scenario in \
  'amd-compute-exhausted 1024 75 24' \
  'amd-memory-exhausted 15360 25 8'
do
  read -r pod_name memory_mib core_percent expected_cus <<<"${scenario}"
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: hami-dra-workloads
spec:
  restartPolicy: Never
  containers:
    - name: workload
      image: registry.k8s.io/pause:3.10
      resources:
        limits:
          amd.com/gpu: "1"
          amd.com/gpumem: "${memory_mib}"
          amd.com/gpucores: "${core_percent}"
EOF

  capacity_claim=$(kubectl get pod -n hami-dra-workloads "${pod_name}" -o jsonpath='{.spec.resourceClaims[*].resourceClaimName}')
  test "$(wc -w <<<"${capacity_claim}" | tr -d ' ')" = 1
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${capacity_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.count}')" = 1
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${capacity_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.deviceClassName}')" = gpu.amd.com
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${capacity_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.selectors}')" = "${amd_selectors}"
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${capacity_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.capacity.requests.computeUnits}')" = "${expected_cus}"
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${capacity_claim}" -o jsonpath='{.spec.devices.requests[0].exactly.capacity.requests.memory}')" = "$((memory_mib * 1024 * 1024))"
  for resource in gpu gpumem gpucores; do
    for field in requests limits; do
      value=$(kubectl get pod -n hami-dra-workloads "${pod_name}" -o jsonpath="{.spec.containers[0].resources.${field}.amd\\.com/${resource}}")
      test -z "${value}"
    done
  done

  kubectl wait -n hami-dra-workloads "pod/${pod_name}" \
    --for='jsonpath={.status.conditions[?(@.type=="PodScheduled")].reason}=Unschedulable' --timeout=60s
  message=$(kubectl get pod -n hami-dra-workloads "${pod_name}" -o jsonpath='{.status.conditions[?(@.type=="PodScheduled")].message}')
  grep -Fq 'cannot allocate all claims' <<<"${message}"
  pod_uid=$(kubectl get pod -n hami-dra-workloads "${pod_name}" -o jsonpath='{.metadata.uid}')

  # Keep observing after the scheduler rejects the request; a single Pending
  # snapshot could otherwise just mean scheduling has not happened yet.
  for ((attempt = 0; attempt < 15; attempt++)); do
    sleep 2
    state=$(kubectl get pod -n hami-dra-workloads "${pod_name}" -o jsonpath='{.status.phase}{"|"}{.spec.nodeName}{"|"}{.status.conditions[?(@.type=="PodScheduled")].status}{"|"}{.status.conditions[?(@.type=="PodScheduled")].reason}')
    test "${state}" = 'Pending||False|Unschedulable'
    allocation=$(kubectl get -n hami-dra-workloads "resourceclaim/${capacity_claim}" -o jsonpath='{.status.allocation}')
    test -z "${allocation}"
  done
  events=$(kubectl get events -n hami-dra-workloads \
    --field-selector "involvedObject.uid=${pod_uid},reason=FailedScheduling" -o jsonpath='{.items[*].message}')
  grep -Fq 'cannot allocate all claims' <<<"${events}"
  printf 'Verified %s: unbound with an unallocated claim for 30 seconds\n' "${pod_name}"

  kubectl delete pod -n hami-dra-workloads "${pod_name}" --wait=true --timeout=120s
  kubectl delete -n hami-dra-workloads "resourceclaim/${capacity_claim}" \
    --ignore-not-found --wait=true --timeout=120s
  test "$(kubectl get pod -n hami-dra-workloads multi-vendor -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')" = True
  test "$(kubectl get -n hami-dra-workloads "resourceclaim/${amd_claim}" -o jsonpath='{.status.allocation.devices.results[0].consumedCapacity.computeUnits}')" = 16
done
