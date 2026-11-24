#!/usr/bin/env just --justfile

XDIR := justfile_directory()
XDIST := XDIR + "/dist"
XTMP := XDIR + "/tmp"
EXE := "samples-composer"
CLI := "sc"
OSARCH := os()+"-"+arch()

## GOARCHS := "linux,amd64 linux,arm64 windows,amd64 windows,arm64 darwin,amd64 darwin,arm64"
GOARCHS := "linux,amd64"

# GOROOT := "/u/fredo/bin/go/go1.26.7"
GOROOT := "/u/fredo/go/go1.27.4"

LXDPLY := "linuxdeploy-x86_64.AppImage"
APPIMG := "appimagetool-x86_64.AppImage"

add-mod _MOD:
    #!/bin/sh -x
    export GOROOT={{GOROOT}}
    export PATH=$GOROOT/bin:$PATH
    go get {{_MOD}}
    go mod vendor

update-mod:
    #!/bin/sh -x
    export GOROOT={{GOROOT}}
    export PATH=$GOROOT/bin:$PATH
    go mod tidy
    go mod vendor

update-version-info:
    #!/bin/sh
    V=$(shtool version -n "{{EXE}}" -d long -l txt ./version.txt)
    echo "$V" >{{XDIR}}/version_info.txt
    echo "package pkg" >{{XDIR}}/pkg/version.go
    echo "const (" >>{{XDIR}}/pkg/version.go
    echo "    PkgVersion = \"$V\"" >>{{XDIR}}/pkg/version.go
    echo ")" >>{{XDIR}}/pkg/version.go

set-drel: do-inc-level
    #!/bin/bash
    V=$(date '+%Y.%m.')
    V=$V$(cd {{XDIR}} && shtool version -n "{{EXE}}" -l short ./version.txt|cut -f 3 -d.)
    cd {{XDIR}} && just -f justfile set-version "$V"

git-push-level: inc-level git-push

git-push: (git-push-msg "upd")
git-push-msg _MSG:
    #!/bin/sh
    VERSION=$(shtool version -l txt ./version.txt)
    TIME=$(date '+%F %T')
    git commit --all -m "{{_MSG}} - v$VERSION on $TIME"
    git push

inc-version: do-inc-version update-version-info
do-inc-version:
    #!/bin/bash
    cd {{XDIR}}
    shtool version -n "{{EXE}}" -i v -l txt ./version.txt

inc-major: do-inc-major update-version-info
do-inc-major:
    #!/bin/bash
    cd {{XDIR}}
    shtool version -n "{{EXE}}" -i r -l txt ./version.txt

inc-level: do-inc-level update-version-info
do-inc-level:
    #!/bin/bash
    cd {{XDIR}}
    shtool version -n "{{EXE}}" -i l -l txt ./version.txt

set-version _VERSION: (do-set-version _VERSION) update-version-info

do-set-version _VERSION:
    #!/bin/bash
    cd {{XDIR}}
    shtool version -n "{{EXE}}" -s "{{_VERSION}}" -l txt ./version.txt

make-update: git-push-level

make-prel: (make-prel-msg "upd")
make-prel-msg _MSG: set-drel git-push
    #!/bin/bash
    VERSION=$(shtool version -l txt ./version.txt)
    VERL=$(shtool version -l text -d long ./version.txt)
    MESSAGE="{{_MSG}} - {{EXE}} automated pre-release version $VERL"
    gh release create v$VERSION --notes "$MESSAGE" --prerelease

make-rel: (make-rel-msg "upd")
make-rel-msg _MSG: set-drel git-push
    #!/bin/bash
    VERSION=$(shtool version -l txt ./version.txt)
    VERL=$(shtool version -l text -d long ./version.txt)
    MESSAGE="{{_MSG}} - {{EXE}} automated release version $VERL"
    gh release create v$VERSION --notes "$MESSAGE"

make-upload:
    #!/bin/bash
    VERSION=$(shtool version -l txt ./version.txt)
    gh release upload v$VERSION {{XDIST}}/{{EXE}}-$VERSION-*

#make-prel-full _MSG: (make-prel-msg _MSG) build-all make-upload
#make-rel-full _MSG: (make-rel-msg _MSG) build-all make-upload
make-prel-full _MSG: (make-prel-msg _MSG) build-all make-upload
make-rel-full _MSG: (make-rel-msg _MSG) build-all make-upload

#build-appimage: build-linux
#    #!/bin/sh
#    rm -rf {{XDIST}}/AppDir; mkdir -p {{XDIST}}/AppDir
#    VERSION=$(shtool version -l txt ./version.txt)
#    cp {{XDIR}}/res/sc.png {{XDIST}}/AppDir/{{CLI}}-linux-amd64.png
#    cp {{XDIR}}/res/AppRun {{XDIST}}/AppDir/AppRun
#    chmod uga+rx {{XDIST}}/AppDir/AppRun
#    {{LXDPLY}} -v0 \
#        --appdir={{XDIST}}/AppDir \
#        -e{{XDIST}}/{{CLI}}-linux-amd64 \
#        --create-desktop-file \
#        --library=/lib/x86_64-linux-gnu/libGL.so.1 \
#        --library=/lib/x86_64-linux-gnu/libffi.so.8 \
#        --library=/lib/x86_64-linux-gnu/libGLdispatch.so.0 \
#        --library=/lib/x86_64-linux-gnu/libGLX.so.0 \
#        --library=/lib/x86_64-linux-gnu/libX11.so.6 \
#        --library=/lib/x86_64-linux-gnu/libxcb.so.1 \
#        --library=/lib/x86_64-linux-gnu/libXau.so.6 \
#        --library=/lib/x86_64-linux-gnu/libXdmcp.so.6 \
#        --library=/lib/x86_64-linux-gnu/libbsd.so.0 \
#        --library=/lib/x86_64-linux-gnu/libmd.so.0
#    {{APPIMG}} {{XDIST}}/AppDir {{XDIST}}/{{EXE}}-$VERSION-x86_64-linux.AppImage
#
##        --library=/lib/x86_64-linux-gnu/libc.so.6 \
##        --library=/lib/x86_64-linux-gnu/libm.so.6 \
##        --library=/lib/x86_64-linux-gnu/libwayland-client.so.0 \
build-all:
    #!/bin/sh
    rm -rf {{XDIST}}; mkdir -p {{XDIST}}
    cd {{XDIR}} && just -f justfile build-docker build-lnx build-win

build-all-inc:  do-inc-level build-all

build-osarch _GOS _GARCH:
    #!/bin/sh
    VERSION=$(shtool version -l txt ./version.txt)
    _gos={{_GOS}}
    _garch={{_GARCH}}
    _gexe=
    case $_gos in
        windows)
            _gexe=.exe
            ;;
        *)
            _gexe=
            ;;
    esac
    #_gout={{CLI}}-${VERSION}-${_gos}-${_garch}${_gexe}
    _gout={{CLI}}-${_gos}-${_garch}${_gexe}
    cd {{XDIR}} && just -f justfile build-osarch-out "${_gos}" "${_garch}" "${_gout}"

build-osarch-out _GOS _GARCH _GOUT: update-version-info
    #!/bin/sh
    export GOROOT={{GOROOT}}
    export PATH=$GOROOT/bin:$PATH
    mkdir -p {{XDIST}}
    #CGO_ENABLED=1 CC=musl-gcc \
    #  go build -ldflags="-linkmode external -extldflags '-static'"
    _gos={{_GOS}}
    _garch={{_GARCH}}
    _gout={{_GOUT}}
    echo "building ${_gos} ${_garch} ... ${_gout}"
    GOOS=${_gos} GOARCH=${_garch} go build -buildmode=pie -o "{{XDIST}}/${_gout}" -v cmd/{{CLI}}/*.go

build-win: update-version-info
    #!/bin/sh -x
    export GOROOT={{GOROOT}}
    export PATH=$GOROOT/bin:$PATH
    mkdir -p {{XDIST}}
    VERSION=$(shtool version -l txt ./version.txt)
    fyne-cross windows \
      -app-id terefang.sc \
      -app-version $VERSION \
      -arch amd64,386 \
      -dir {{XDIR}} \
      -icon res/sc.png \
      -name "{{EXE}}" \
      -output "{{EXE}}-$VERSION" \
      cmd/sc
    cp {{XDIR}}/fyne-cross/bin/windows-amd64/{{EXE}}-$VERSION.exe {{XDIST}}/{{EXE}}-$VERSION-windows-amd64.exe
    cp {{XDIR}}/fyne-cross/bin/windows-386/{{EXE}}-$VERSION.exe {{XDIST}}/{{EXE}}-$VERSION-windows-x86.exe

build-mac: update-version-info
    #!/bin/sh -x
    export GOROOT={{GOROOT}}
    export PATH=$GOROOT/bin:$PATH
    mkdir -p {{XDIST}}
    VERSION=$(shtool version -l txt ./version.txt)
    fyne-cross darwin \
      -app-id terefang.sc \
      -app-version $VERSION \
      -arch arm64 \
      -dir {{XDIR}} \
      -icon res/sc.png \
      -name "{{EXE}}" \
      -output "{{EXE}}-$VERSION" \
      cmd/sc
    cp {{XDIR}}/fyne-cross/bin/darwin-arm64/{{EXE}}-$VERSION {{XDIST}}/{{EXE}}-$VERSION-darwin-arm64

build-linux: (build-osarch "linux" "amd64")
build-lnx: update-version-info
    #!/bin/sh -x
    . /etc/os-release
    OSTAG="$ID-$VERSION_ID-$VERSION_CODENAME"
    export GOROOT={{GOROOT}}
    export PATH=$GOROOT/bin:$PATH
    mkdir -p {{XDIST}}
    VERSION=$(shtool version -l txt ./version.txt)
    fyne-cross linux \
      -app-id terefang.sc \
      -app-version $VERSION \
      -arch amd64 \
      -dir {{XDIR}} \
      -icon res/sc.png \
      -name "{{EXE}}" \
      -output "{{EXE}}-$VERSION" \
      cmd/sc
    cp {{XDIR}}/fyne-cross/bin/linux-amd64/{{CLI}} {{XDIST}}/{{EXE}}-$VERSION-linux-amd64-$OSTAG

render-docker-template _TEMPLATE _VARIANT _IMAGE:
    #!/bin/sh -x
    cd {{XDIR}}
    XVERSION=$(shtool version -l txt ./version.txt)
    XARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
    DOCKERIMAGE="{{_IMAGE}}" template -o docker/Dockerfile.{{_VARIANT}}.TMP docker/Dockerfile.{{_TEMPLATE}}.tmpl

build-docker-debian9: \
    (render-docker-template "debian_x11" "debian9" "debian/eol:stretch-slim") \
    (build-docker-variant "debian9" "compat-glibc-2.24-x11-only")

build-docker-debian10: \
    (render-docker-template "debian_x11" "debian10" "debian:buster-slim") \
    (build-docker-variant "debian10" "compat-glibc-2.28-x11-only")

build-docker-debian12: \
    (render-docker-template "debian" "debian12" "debian:bookworm-slim") \
    (build-docker-variant "debian12" "compat-glibc-2.36")

build-docker-debian13: \
    (render-docker-template "debian" "debian13" "debian:trixie-slim") \
    (build-docker-variant "debian13" "compat-glibc-2.41")

build-docker-ubuntu18: \
    (render-docker-template "ubuntu_x11" "ubuntu18" "ubuntu:bionic") \
    (build-docker-variant "ubuntu18" "")

build-docker-ubuntu20: \
    (render-docker-template "ubuntu_x11" "ubuntu20" "ubuntu:focal") \
    (build-docker-variant "ubuntu20" "")

build-docker-ubuntu22: \
    (render-docker-template "ubuntu" "ubuntu22" "ubuntu:jammy") \
    (build-docker-variant "ubuntu22" "")

build-docker-ubuntu24: \
    (render-docker-template "ubuntu" "ubuntu24" "ubuntu:noble") \
    (build-docker-variant "ubuntu24" "")

build-docker-ubuntu26: \
    (render-docker-template "ubuntu" "ubuntu26" "ubuntu:stonking") \
    (build-docker-variant "ubuntu26" "")

build-docker-fedora45: \
    (render-docker-template "rhel" "fedora45" "fedora:45") \
    (build-docker-variant "fedora45" "")

build-docker-fedora44: \
    (render-docker-template "rhel" "fedora44" "fedora:44") \
    (build-docker-variant "fedora44" "")

build-docker-fedora43: \
    (render-docker-template "rhel" "fedora43" "fedora:43") \
    (build-docker-variant "fedora43" "")

build-docker-fedora42: \
    (render-docker-template "rhel" "fedora42" "fedora:42") \
    (build-docker-variant "fedora42" "")

build-docker-fedora: \
    build-docker-fedora42 \
    build-docker-fedora43 \
    build-docker-fedora44 \
    build-docker-fedora45

build-docker-debian: \
    build-docker-debian9 \
    build-docker-debian10 \
    build-docker-debian12 \
    build-docker-debian13

build-docker-ubuntu: \
    build-docker-ubuntu18 \
    build-docker-ubuntu20 \
    build-docker-ubuntu22 \
    build-docker-ubuntu24 \
    build-docker-ubuntu26

build-docker-arch: \
    (render-docker-template "arch" "archlinux" "archlinux/archlinux") \
    (build-docker-variant "archlinux" "")

build-docker: build-docker-debian build-docker-ubuntu

build-docker-variant _VARIANT _ALIAS:
    #!/bin/sh
    cd {{XDIR}}
    XVERSION=$(shtool version -l txt ./version.txt)
    XARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
    docker builder prune -f
    docker build --progress=plain -f docker/Dockerfile.{{_VARIANT}}.TMP -t fyne-oldlinux-{{_VARIANT}} . && \
    docker create --name fyne-extract-{{_VARIANT}} fyne-oldlinux-{{_VARIANT}} && \
    docker cp fyne-extract-{{_VARIANT}}:/fyne-app {{XDIST}}/{{EXE}}-$XVERSION-linux-$XARCH-{{_VARIANT}} && \
    docker rm fyne-extract-{{_VARIANT}} && \
    docker image rm fyne-oldlinux-{{_VARIANT}}
    ( [ -n "{{_ALIAS}}" -a -f "{{XDIST}}/{{EXE}}-$XVERSION-linux-$XARCH-{{_VARIANT}}" ] && cp {{XDIST}}/{{EXE}}-$XVERSION-linux-$XARCH-{{_VARIANT}} {{XDIST}}/{{EXE}}-$XVERSION-linux-$XARCH-{{_ALIAS}} ) || true

