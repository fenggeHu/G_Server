FROM ubuntu:24.04
ARG GODOT_VERSION=4.7.2
ARG GODOT_RELEASE=stable
ARG GODOT_SHA256=
WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl unzip \
    && rm -rf /var/lib/apt/lists/* \
    && curl -fsSL -o /tmp/godot.zip "https://github.com/godotengine/godot/releases/download/${GODOT_VERSION}-${GODOT_RELEASE}/Godot_v${GODOT_VERSION}-${GODOT_RELEASE}_linux.x86_64.zip" \
    && if [ -n "$GODOT_SHA256" ]; then echo "$GODOT_SHA256  /tmp/godot.zip" | sha256sum -c -; fi \
    && unzip /tmp/godot.zip -d /usr/local/bin \
    && mv /usr/local/bin/Godot_v${GODOT_VERSION}-${GODOT_RELEASE}_linux.x86_64 /usr/local/bin/godot \
    && chmod +x /usr/local/bin/godot \
    && rm /tmp/godot.zip
COPY GameServer/ /app/
ENTRYPOINT ["godot", "--headless", "--path", "/app", "--", "--server"]
