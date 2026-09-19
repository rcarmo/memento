FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251
RUN apt-get update && apt-get install -y --no-install-recommends python3 libvulkan1 mesa-vulkan-drivers vulkan-tools && rm -rf /var/lib/apt/lists/*
COPY memento-embed gte-small.gtemodel vulkan_pretest.py /opt/pretest/
RUN dpkg-query -W libvulkan1 mesa-vulkan-drivers vulkan-tools libdrm2 python3 > /opt/pretest/packages.txt && sha256sum /opt/pretest/memento-embed /opt/pretest/gte-small.gtemodel /opt/pretest/vulkan_pretest.py > /opt/pretest/SHA256SUMS
USER 65532:65532
ENV HOME=/tmp XDG_RUNTIME_DIR=/tmp PYTHONDONTWRITEBYTECODE=1 VK_ICD_FILENAMES=/usr/share/vulkan/icd.d/intel_icd.x86_64.json VK_DRIVER_FILES=/usr/share/vulkan/icd.d/intel_icd.x86_64.json RAYON_NUM_THREADS=1 OMP_NUM_THREADS=1 OPENBLAS_NUM_THREADS=1 MKL_NUM_THREADS=1
ENTRYPOINT ["/bin/sh"]
