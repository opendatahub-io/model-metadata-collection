# Simplified Dockerfile for model metadata collection
# Since models-catalog.yaml is now checked in by CI/CD, we only need to copy the files
FROM registry.access.redhat.com/ubi9-micro:latest

# Create directories for mounting by other applications
RUN mkdir -p /app/data /app/benchmarks

# Copy the pre-generated catalog and index files from the codebase
COPY data/models-catalog.yaml /app/data/
COPY data/models-index.yaml /app/data/
COPY data/validated-models-catalog.yaml /app/data
COPY data/validated-models-index.yaml /app/data

# Copy sample data for benchmarks
COPY sample-data/ /app/benchmarks/

# Set proper permissions
RUN chmod 644 /app/data/models-catalog.yaml /app/data/models-index.yaml && \
    chmod 644 /app/data/validated-models-catalog.yaml /app/data/validated-models-index.yaml && \
    chmod -R 755 /app/benchmarks && \
    chmod 644 /app/benchmarks/manifest.json && \
    /bin/sh -c 'for f in /app/benchmarks/models/*/*/*/*; do [ -f "$f" ] && chmod 644 "$f" || true; done'

# Create a non-root user for security
RUN echo "catalog:x:1001:1001:Catalog User:/app:/sbin/nologin" >> /etc/passwd && \
    echo "catalog:x:1001:" >> /etc/group && \
    chown -R 1001:1001 /app

# Switch to non-root user
USER 1001

# Set working directory
WORKDIR /app

# Volume mount points for external applications
VOLUME ["/app/data", "/app/benchmarks"]

# Default command - just keep container running for data access
CMD ["sleep", "infinity"]

# Labels for image metadata
LABEL name="model-metadata-collection" \
      version="1.0" \
      description="Model metadata collection catalog generator" \
      io.k8s.description="Generates and serves model metadata catalog" \
      io.k8s.display-name="Model Metadata Collection" \
      io.openshift.tags="ai,models,metadata,catalog"
