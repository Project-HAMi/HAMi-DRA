# Changelog

## v0.2.4

### Added

- AMD GPUs: `amd.com/gpu`, `amd.com/gpumem`, `amd.com/gpumem-percentage` and `amd.com/gpucores` requests are converted for the AMD DRA driver (`deviceVendors` with `amd`, `amdReferenceComputeUnits`), and `drivers.amd.enabled` installs the driver on nodes labelled `hami.io/amd-mode=dra` (#172). See [docs/amd-dra.md](docs/amd-dra.md).
- `amd.com/gpucores` works without `amdReferenceComputeUnits` when every published AMD GPU has the same `computeUnits` (#202).
- A pod can opt out of the DRA conversion with the label `hami.io/skip-dra` (#182).
- Monitor metric names and labels follow HAMi and Prometheus conventions; `monitor.legacyMetrics` (on by default) keeps the old names too (#160).
- The multi-vendor e2e suite covers AMD allocation (#198).

### Changed

- The AMD DRA driver image is `0.4.1`, which names Instinct GPUs by device ID (the MI325X was published as `AMD_Radeon_Graphics`).
- The webhook, monitor and fake driver images are `v0.2.4`.

### Fixed

- The chart sets a seccomp profile so the webhook and the monitor run under the restricted Pod Security level (#181).
- Webhook ownership checks, multi-slice cache, chart probes and the CA bundle.
