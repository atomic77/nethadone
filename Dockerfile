FROM golang:1.22 AS build

WORKDIR /go/src/nethadone

# Copy all the Code and stuff to compile everything
COPY . .

# Downloads all the dependencies in advance (could be left out, but it's more clear this way)
RUN go mod download

RUN make build

FROM ubuntu:jammy as release

WORKDIR /app

# This is a lot of stuff to keep around in the release image; will need to determine what really needs
# to be here for typical use
RUN apt-get update -y && \
    apt-get install -y apt-transport-https ca-certificates curl clang llvm jq cmake \
        libelf-dev libpcap-dev binutils-dev build-essential make libbpf-dev \
        python3-pip vim git \
        avahi-daemon python-is-python3 gcc-multilib \
        python3-dnslib python3-cachetools ca-certificates

RUN apt-get install -y linux-headers-generic gcc-multilib 

# Jammy requirements per iovisor/bcc README
RUN apt install -y zip bison build-essential cmake flex git libedit-dev \
  libllvm14 llvm-14-dev libclang-14-dev python3 zlib1g-dev libelf-dev libfl-dev python3-setuptools \
  liblzma-dev libdebuginfod-dev arping netperf iperf

# Build bpftool 
RUN mkdir /src && \
    cd /src && git clone --depth 1 -b v7.4.0 --recurse-submodules https://github.com/libbpf/bpftool.git && \
    cd bpftool/src && make -j install

# Build bcc
RUN cd /src && git clone --depth 1 -b v0.29.1 https://github.com/iovisor/bcc.git && \
    mkdir bcc/build && cd bcc/build && \
    cmake .. && make -j install

# TODO Figure out what we can clean up from the src build directories

# Create the `public` dir and copy all the assets into it
RUN mkdir ./static ./views ./ebpf
COPY ./static ./static
COPY ./views ./views
COPY ./ebpf ./ebpf

COPY --from=build /go/src/nethadone/nethadone .
RUN chmod +x /app/nethadone

# Exposes port 3000 because our program listens on that port
EXPOSE 3000

ENTRYPOINT ["/app/nethadone"]
