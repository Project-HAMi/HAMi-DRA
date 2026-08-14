---
name: Bug Report
about: Report a problem encountered while using HAMi-DRA
labels: bug
---

<!-- Please use this template while reporting a bug and provide as much info as possible. Not doing so may result in your bug not being addressed in a timely manner. Thanks!
-->

**What happened**:

**What you expected to happen**:

**How to reproduce it (as minimally and precisely as possible)**:

**Anything else we need to know?**:

- Relevant, time-bounded excerpts from the HAMi-DRA webhook and monitor container logs
- Relevant, time-bounded excerpts from the API server, scheduler, container runtime, and kubelet logs
- Relevant, redacted Pod, ResourceClaim, ResourceClaimTemplate, and device-class manifests or events
- Relevant Helm values and CDI or container-runtime configuration sections
- Relevant, time-bounded device-driver or kernel output

Before posting, remove or mask credentials, tokens, certificate private keys, GPU or device identifiers, Pod and ResourceClaim identifiers, namespace or node names, internal endpoints, and other sensitive data from configuration and logs.

**Environment**:
- HAMi-DRA version, commit, image, or Helm chart version:
- Kubernetes version and enabled DRA feature gates:
- HAMi version:
- Device type and driver version:
- Container runtime version:
- cert-manager version or custom certificate configuration:
- Kernel version from `uname -a`:
- Others:
