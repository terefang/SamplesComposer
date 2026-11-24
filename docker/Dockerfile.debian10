# syntax=docker/dockerfile:1
FROM debian:buster-slim

ENV DEBIAN_FRONTEND=noninteractive
ENV GO_VERSION=1.26.7
ENV PATH="/usr/local/go/bin:${PATH}"

# Debian buster is EOL: point apt to archive.debian.org
#RUN sed -i 's|http://deb.debian.org/debian|http://ftp.at.debian.org/debian-archive/|g' /etc/apt/sources.list
#RUN sed -i 's|http://security.debian.org/debian-security|http://ftp.at.debian.org/debian-security|g' /etc/apt/sources.list
RUN sed -i 's|/deb.debian.org/|/ftp.at.debian.org/|g' /etc/apt/sources.list
RUN echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99no-check-valid
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gcc \
        g++ \
        libc6-dev \
        pkg-config \
        patchelf \
        libgl1-mesa-dev \
        libegl1-mesa-dev \
        libxkbcommon-dev \
        libx11-dev \
        libxcursor-dev \
        libxrandr-dev \
        libxinerama-dev \
        libxi-dev \
        libxxf86vm-dev

RUN rm -rf /var/lib/apt/lists/*


# Install a modern Go toolchain
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') && \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" | tar -C /usr/local -xzf -


WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download


COPY . .
RUN rm -rf vendor && go mod tidy && go mod vendor
# Build vendored GLFW v3.4 with X11 only (no Wayland headers or runtime libs)
RUN if [ -d vendor/github.com/go-gl/glfw/v3.4/glfw ]; then \
        rm -vf vendor/github.com/go-gl/glfw/v3.4/glfw/c_glfw_lin_wayland.go \
              vendor/github.com/go-gl/glfw/v3.4/glfw/glfw/src/wl_*.c && \
        sed -i 's/#define _GLFW_WAYLAND/\/\/ #define _GLFW_WAYLAND/g' \
            vendor/github.com/go-gl/glfw/v3.4/glfw/glfw/src/platform.h || true; \
    fi
# Provide no-op stubs for Wayland symbols so linking succeeds on Buster (libwayland 1.16)
RUN cp -vf res/c_glfw_lin_wayland.go vendor/github.com/go-gl/glfw/v3.4/glfw/c_glfw_lin_wayland.go
#build
ENV CGO_ENABLED=1 GOOS=linux
RUN ls -la && pwd && go build -v -tags no_wayland,glfw_no_wayland -trimpath -o /out/fyne-app ./cmd/sc
RUN patchelf --remove-needed "libwayland-client.so.0" /out/fyne-app || true
RUN patchelf --remove-needed "libwayland-cursor.so.0" /out/fyne-app || true
RUN patchelf --remove-needed "libwayland-egl.so.0" /out/fyne-app || true
RUN patchelf --remove-needed "libwayland-egl.so.1" /out/fyne-app || true


FROM scratch
COPY --from=0 /out/fyne-app /fyne-app
ENTRYPOINT ["/fyne-app"]