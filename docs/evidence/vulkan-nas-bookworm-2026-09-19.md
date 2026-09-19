# NAS Vulkan works with container-only Mesa 22

Debian Bookworm's unmodified Mesa 22.3.6 runs the existing FP32 GTE1 model on the NAS's Intel HD Graphics 500, without changing the host kernel or production Memento. All four tested CPU/Vulkan parity cases passed. Cold Vulkan was slower in every case, so this establishes hardware compatibility, not a reason to switch production away from CPU.

This follows the [Mesa 26 compatibility failure](vulkan-nas-2026-09-19.md) and Rui's instruction to test the container-only workaround in [#39](https://github.com/rcarmo/memento/issues/39). It supersedes the earlier blocker for this older userspace stack only.

## NAS decision: CPU only

Rui decided against Vulkan on the NAS after the later same-process warm test. Completed three-repeat warm medians were CPU/Vulkan 0.212/8.085 seconds (short), 0.680/8.844 (mixed batch) and 3.072/12.004 (medium). CPU token-limit median was 19.245 seconds; two Vulkan samples took 24.784 and 25.257 seconds before the user stopped the run. Startup was separately timed at 0.291 seconds CPU and 12.322 seconds Vulkan. The same PID was retained for each backend's completed calls; recorded GPU parity passed.

The test was stopped immediately and its container/imported image removed. Exit 137 reflects the requested stop; `OOMKilled` was false. Idle-120-second snapshots, expiry and restart were not reached, so the interrupted experiment does not qualify them. Production remained unchanged and healthy with zero restarts. [Issue #39](https://github.com/rcarmo/memento/issues/39#issuecomment-5745080164) is closed as not planned. This CPU-only decision also applies to the Go port on this NAS; retained recipes are historical references, not deployment instructions. Other GPU targets are unaffected. Further NAS Vulkan work requires a new explicit request.

## Exact stack and isolation

The test ran from 15:02:16 to 15:04:26 UTC on 2026-09-19. The NAS kept kernel `4.4.302+`, its existing i915 driver and PCI device `8086:5a85`. Vulkan identified `Intel(R) HD Graphics 500 (APL 2)`, an integrated GPU, with device API 1.3.230 and Mesa 22.3.6.

The image was built locally without exposing a GPU, then loaded into the NAS through the Docker API. It used distro packages rather than a patched driver:

| Component | Version |
| --- | --- |
| Base | `debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251` |
| Mesa Vulkan drivers | `22.3.6-1+deb12u2` |
| Vulkan loader | `1.3.239.0-1` |
| Vulkan tools | `1.3.239.0+dfsg1-1` |
| libdrm2 | `2.4.114-1+b1` |
| Test image ID | `sha256:c4f3a3a73d302a8d3661a3f6116c5cfc14d5c5c1f84402c62832d040a0d87d7e` |

The previously checksummed worker/model bundle supplied `memento-embed` from commit `d4b72524020e5b02c93862a9dbec328b3e4b6e0a`. The Rust code and pre-test script are unchanged in follow-up commit `b30389ca65ec0dd8b453324a65ccd6966b96f401`; this run does not test the newer Python warm-worker lifecycle.

* Worker SHA-256: `03c7feb68fd0e4dd93001fc7c1ddd5508f1d1635d10773478d2c8d4991b8ca75`.
* Exact GTE1 model SHA-256: `06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171`.
* Runner SHA-256: `a04041be3ccc604095a12e257d576ee08d67951ddac02d83f2bdcb2f1a1ad2ba`.

The container ran as UID/GID 65532 with supplementary render group 937. Only `/dev/dri/renderD128` was mapped read/write. It had no network or application-data mounts, a read-only root, all capabilities dropped and `no-new-privileges`. Memory and memory-plus-swap limits were both 512 MiB, with a 32 MiB `/tmp` tmpfs. CPU affinity was restricted to core 3, low CPU shares were set, and native thread pools were limited to one thread. The NAS lacks enforceable CPU-CFS/PIDs-limit support; those controls are not claimed.

Both Vulkan driver-selection environment variables explicitly selected `intel_icd.x86_64.json`. Software ICDs could not stand in for Intel hardware. Hardware enumeration had a 45-second timeout; each worker had 120 seconds and the whole GTE test 600 seconds, with bounded termination grace. No checks were disabled or spoofed.

## Same-model result

The unchanged pre-test ran one cold sample per case/backend and a separate absent-device auto-fallback check. Each explicit Vulkan response identified the Intel GPU with no fallback. Successful inference also exercised device creation, WGSL pipeline compilation and the startup same-model self-test.

| Case | CPU seconds | Vulkan seconds |
| --- | ---: | ---: |
| Short | 0.347 | 17.365 |
| Unicode/empty batch of three | 0.830 | 18.031 |
| Medium | 3.200 | 20.799 |
| Token limit | 20.126 | 33.736 |

Maximum absolute vector error was `1.7881393432617188e-7`; minimum cosine was `0.9999999999996154`. The runner checked finite values and unit norms. Missing-device auto mode selected CPU as expected. Both `vulkaninfo` and the GTE runner exited zero; Docker recorded exit zero and `OOMKilled: false`.

These are single cold observations, including model loading, device/pipeline setup and the startup self-test, not repeated timing medians or warm measurements. Do not subtract CPU from Vulkan time to claim a measured startup cost. Raw vector payloads were not retained by the existing runner, so later report verification checks recorded parity outcomes rather than independently recomputing vectors.

## Memory and preserved production

The test cgroup's recorded peak was 409,124,864 bytes (**390.172 MiB**); `memory.failcnt` was zero. This passes the 512 MiB cap for the standalone runner and its sequential workers. It does not establish that the full Memento daemon plus a worker fits that cap under production load, nor that persistent Vulkan allocations are safe to retain indefinitely. The cgroup peak is not a complete device-memory accounting measure.

The test container and imported test image were removed from the NAS afterwards. No anonymous volume was created. Production inspection before/after confirmed the same container, image, complete configuration, host settings, mount mappings and start time. Memento remained healthy with zero restarts; its embedding worker was alive/available, had no last error and reported 74 completed jobs. The test never mounted production data/model/configuration volumes; their individual contents were not rehashed.

No host installation, driver replacement, live deployment, reserved local GPU work or Sigma retest occurred. Build and transfer artefacts remain local for reproduction.

## Go-port handoff

Rui requested retaining this route for the Go port. The verified component is the container userspace stack and NAS device access, independent of the language of the eventual client. The [exact test Dockerfile](nas-bookworm-pretest.Dockerfile) is retained with this report. It pins the base image but used the Bookworm package repositories at build time; its package versions are recorded above, not locked by that Dockerfile. A release recipe must lock or snapshot the full dependency set and recheck security updates rather than assuming a later rebuild is byte-identical.

Use the same GTE1 bytes, tokenisation, FP32 computation and pooling for comparisons. A Go process may reuse the framed Rust worker if that fits the port, or qualify its own backend against the same reference; this result does not choose the Go architecture or prove Go-backend compatibility. Preserve CPU default, explicit hardware selection/software-adapter rejection, fail-closed Vulkan, bounded whole-batch auto fallback and experimental embedding identity.

Device group 937 is specific to this NAS and must be discovered on other hosts. A Go image needs the Vulkan loader/Intel ICD and their coherent userspace dependencies, not Python merely to use Vulkan; Python here runs the reference test. Repeat parity, startup/warm timing and full-service memory checks with the actual Go build. Do not enable Vulkan by default based on enumeration alone: every cold GPU case was slower, and 390.172 MiB is the standalone test peak, not a Go-service memory budget.

## What changes next

There is now a verified container-only compatibility route, so a host upgrade is not required to demonstrate NAS hardware GTE support. The original Mesa 26 message also needs care: its source uses the same capture-support error for capture and timeline-fence checks; the earlier line 119 matches the upstream timeline check. Older Mesa retains optional paths for those capabilities.

Keep CPU as production default. A separate bounded warm NAS experiment would need to measure startup versus subsequent requests, idle memory and reclamation before deciding whether reuse makes this GPU worthwhile. The present result does not authorise production enabling or lift the reserved local GPU hold.

The [machine-readable result](vulkan-nas-bookworm-2026-09-19.json) contains samples, identities, bounds, cleanup checks and source hashes. Local originals, Dockerfile, logs and API records are under `notes/projects/memento-nas-bookworm-2026-09-19/`.
