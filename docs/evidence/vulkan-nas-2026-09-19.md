# NAS Vulkan compatibility gate

The DiskStation's Intel GPU is present and accessible through a mapped render node, but Mesa 26.0.3 cannot initialise hardware Vulkan on its current `4.4.302+` kernel. The Intel driver reports **kernel missing exec capture support**. No GTE inference ran: the prerequisite adapter-enumeration gate failed.

This answers the first stage of [#39](https://github.com/rcarmo/memento/issues/39), after Rui requested execution on 2026-09-19. It is an unsupported tested driver/kernel combination, not proof that the GPU could never support Vulkan with another stack. No driver replacement, host upgrade or production deployment was attempted.

## Candidate and target

The optional backend and warm-worker candidate were at `ae3883840b6d2815b0a8728b5077952b1ceced1b` (draft PR #38, stacked on #36). Both PRs had green Python 3.12--3.14, optional Vulkan CPU-safe and container checks. The candidate binary was not copied to the NAS because adapter enumeration failed first.

The NAS reports four CPU cores, Intel PCI `8086:5a85` at `0000:00:02.0`, bound to `i915`. Sysfs contains `card0` and `renderD128`. The render node is mode `0660`, owner root, group 937. Production Memento has no GPU mapping or Vulkan ICD.

A disposable container reused an image already present on the NAS, without running its normal entrypoint or mounting any application data:

* Image ID: `sha256:74132defe387df722a638326ba6c300db1475471d6c2e19036fbb04674500f3c` (installed Calibre runtime image).
* Vulkan loader/tools: 1.4.341; Mesa Vulkan drivers: `26.0.3-1ubuntu1`.
* Intel ICD: `libvulkan_intel.so`, advertised API 1.4.335; legacy Intel ICD: `libvulkan_intel_hasvk.so`, advertised API 1.3.335.
* UID/GID 65532, supplementary device group 937, all capabilities dropped, `no-new-privileges`, read-only root, no network, no host/application mounts, only `renderD128` mapped read/write.
* Enumeration limit: 512 MiB including swap allowance (no additional swap), CPU affinity to one core and low CPU shares; 32 MiB `/tmp` and 1 MiB `/config` tmpfs. Each `vulkaninfo` invocation had a 45-second timeout plus five seconds for termination. The NAS Docker daemon reports CPU CFS quota and PIDs-limit support absent, so no claim is made that those controls were enforced.

The preliminary access-only container used 256 MiB and no supplementary device group. It inspected device metadata and available libraries without opening the GPU. Adding the existing render group to the disposable enumeration container was an explicit device grant, not a host permission change.

## Driver result

`VK_DRIVER_FILES` selected one Intel ICD at a time; software ICDs were excluded.

```text
ICD=intel
kernel missing exec capture support (VK_ERROR_INITIALIZATION_FAILED)
vkEnumeratePhysicalDevices ... failed with error code -3
Failed to detect any valid GPUs in the current config
vkEnumeratePhysicalDevices failed with ERROR_INITIALIZATION_FAILED
vulkaninfo_exit=1

ICD=intel_hasvk
Failed to detect any valid GPUs in the current config
vkEnumeratePhysicalDevices failed with ERROR_INITIALIZATION_FAILED
vulkaninfo_exit=1
```

The modern Intel ICD gives the concrete kernel-feature diagnostic. The legacy ICD also found no usable device; it does not establish an alternative workaround. Loader/ICD advertised API versions are not evidence of a successfully created hardware adapter.

The wrapper container exited zero because it collected both commands' statuses. **Both actual Vulkan commands exited one.** Container `OOMKilled` was false; there are no GTE memory/timing measurements and no 512 MiB inference qualification.

An initial access script stopped on `id` returning one for an unmapped numeric username; it was corrected before GPU enumeration. Some Portainer create/start requests exceeded the add-on's 30-second request deadline. State was inspected before retries. The enumeration container remained unstarted with `context canceled`; a bounded 180-second API start completed with HTTP 204 in about 64 seconds. This was container startup latency, not GPU inference time.

## Production and cleanup

All three disposable probe containers were removed. The first image-created anonymous `/config` volume was removed with its container and returned HTTP 404 on verification; later probes used tmpfs at that path. No probe containers remained in endpoint inventory.

Before/after inspection confirmed production Memento retained the same container, image, complete container configuration (including environment), host configuration, mount mappings and start time. Mount arrays were compared by destination, not incidental ordering. It stayed healthy with zero restarts. Its embedding status returned HTTP 200 with `alive: true`, `available: true`, `last_error: null` and 74 completed jobs.

The probes did not mount or write the production repository, databases, model volume, configuration, secrets, proposals, assets or vectors. Their content was not independently rehashed during this check. No local GSQ GPU work or Sigma retest occurred.

The checked results and source-file hashes are in [the machine-readable report](vulkan-nas-2026-09-19.json). The original API responses and decoded log are retained locally under `notes/projects/memento-nas-vulkan-2026-09-19/`.

## Remaining work

Keep CPU on this NAS. An alternative supported driver/kernel combination would need a separately reviewed, non-invasive qualification path before retrying. Do not downgrade drivers or modify the host merely to bypass this result. Until a hardware adapter initialises, GTE parity, useful speedup, warm memory retention/release and production-cap suitability remain blocked.
