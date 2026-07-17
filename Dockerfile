FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    TMPDIR=/var/lib/memento/tmp

RUN addgroup --system --gid 65532 memento \
    && adduser --system --uid 65532 --ingroup memento memento \
    && mkdir -p /app /var/lib/memento/tmp \
    && chown -R memento:memento /app /var/lib/memento

WORKDIR /app
COPY pyproject.toml README.md /app/
COPY src /app/src
RUN python -m pip install --upgrade pip \
    && python -m pip install .

USER 65532:65532
VOLUME ["/var/lib/memento"]
ENTRYPOINT ["memento-serve"]
CMD ["--help"]
