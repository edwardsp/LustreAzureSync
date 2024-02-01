FROM mcr.microsoft.com/mirror/docker/library/ubuntu:20.04

SHELL ["/bin/bash", "-c"]

# It would be great to combine all these ENV into one block, but they aren't interpolated immediately.
ENV DISTRIB_CODENAME="focal" \
    TZ="Etc/UTC" \
    DEBIAN_FRONTEND="noninteractive" \
    \
    USR_INCLUDE="/usr/include" \
    USR_LIB="/usr/lib" \
    USR_LOCAL="/usr/local" \
    \
    LUSTRE_VER="2.15.1-24-gbaa21ca"

ENV LUSTRE_CLIENT_DEB="lustre-client_${LUSTRE_VER}_amd64.deb" \
    UAPI_INC_DIR="${USR_INCLUDE}/lustre/uapi" \
    \
    LEMUR_SRC_DIR="/usr/src/lemur" \
    TEMP_DIR="/tmp/lustre" \
    \
    GO_PKG="go1.21.4.linux-amd64.tar.gz" \
    GO_BIN_FOLDER="${USR_LOCAL}/go"

# These setup the go flags for building lemur.
ENV PATH="$PATH:${GO_BIN_FOLDER}/bin" \
    CGO_CFLAGS="-I${USR_INCLUDE} -I${UAPI_INC_DIR}" \
    CGO_LDFLAGS="-L${USR_LIB}" \
    GOCACHE="${LEMUR_SRC_DIR}/.cache-go/go-build" \
    GOPATH="${LEMUR_SRC_DIR}/go"

# Switch to Azure repos for Azure packages and install pre-req packages.
RUN    sed -i -e "s|archive\.ubuntu\.com|azure\.archive\.ubuntu\.com|g" /etc/apt/sources.list \
    && sed -i -e "s|security\.ubuntu\.com|azure\.archive\.ubuntu\.com|g" /etc/apt/sources.list \
    && sed -i '/partner$/d' /etc/apt/sources.list \
    && sed -i '/deb-src/s/^# //' /etc/apt/sources.list \
    && apt-get update \
    && apt-get install -y wget kmod make git gcc libreadline8

# Install go.
RUN    wget "https://go.dev/dl/${GO_PKG}" \
    && rm -rf "${GO_BIN_FOLDER}" \
    && tar -C "${USR_LOCAL}" -xzf "${GO_PKG}"

# Download and extract the userspace deb from PMC (client util + devel).
RUN    mkdir -p "${TEMP_DIR}" \
    && wget "https://packages.microsoft.com/repos/amlfs-${DISTRIB_CODENAME}/pool/main/l/lustre-client/${LUSTRE_CLIENT_DEB}" \
            -O "${TEMP_DIR}/${LUSTRE_CLIENT_DEB}" \
    && dpkg-deb -xv "${TEMP_DIR}/${LUSTRE_CLIENT_DEB}" "/"

# Cleanup to shrink image.
RUN    rm -rf "${TEMP_DIR}" \
    && apt-get clean