FROM ubuntu:24.04
ARG GODOT_VERSION=4.7.2
ARG GODOT_RELEASE=stable
ARG GODOT_SHA256=
ARG TARGETARCH
RUN case "$TARGETARCH" in amd64) echo x86_64 > /tmp/godot_arch ;; arm64) echo arm64 > /tmp/godot_arch ;; *) echo "unsupported architecture: $TARGETARCH" >&2; exit 1 ;; esac
WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl unzip libfontconfig1 \
    && rm -rf /var/lib/apt/lists/* \
    && arch=$(cat /tmp/godot_arch) \
    && curl -fsSL -o /tmp/godot.zip "https://github.com/godotengine/godot/releases/download/${GODOT_VERSION}-${GODOT_RELEASE}/Godot_v${GODOT_VERSION}-${GODOT_RELEASE}_linux.${arch}.zip" \
    && if [ -n "$GODOT_SHA256" ]; then echo "$GODOT_SHA256  /tmp/godot.zip" | sha256sum -c -; fi \
    && unzip /tmp/godot.zip -d /usr/local/bin \
    && mv /usr/local/bin/Godot_v${GODOT_VERSION}-${GODOT_RELEASE}_linux.$(cat /tmp/godot_arch) /usr/local/bin/godot \
    && chmod +x /usr/local/bin/godot \
    && rm /tmp/godot.zip
COPY GameServer/ /app/
ENTRYPOINT ["godot", "--headless", "--path", "/app", "--", "--server"]
