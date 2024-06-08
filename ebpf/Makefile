TARGET = tcfilt

BPF_OBJ = ${TARGET:=.bpf.o}

all: $(TARGET) $(BPF_OBJ) 
.PHONY: all 
.PHONY: $(TARGET)

# bpftool prog load $(BPF_OBJ) /sys/fs/bpf/$(TARGET)
# bpftool net attach xdp pinned /sys/fs/bpf/$(TARGET) dev lo
# bpftool net detach xdp dev lo

$(TARGET): $(BPF_OBJ)
	sudo rm -f /sys/fs/bpf/$(TARGET)

$(BPF_OBJ): %.o: %.c vmlinux.h
	clang \
	    -target bpf \
	    -D __BPF_TRACING__ \
		-I/usr/include/$(shell uname -m)-linux-gnu \
	    -Wall \
	    -O2 -o $@ -c $<

vmlinux.h:
	bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h		



rtarg = pkt
direction = ingress
#rtarg = flat
dev = eth1
build: 
	clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c ${rtarg}.bpf.c -o ${rtarg}.o
rebuild:
	tc filter del dev ${dev} ${direction}
	clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c ${rtarg}.bpf.c -o ${rtarg}.o
	tc filter add dev ${dev} ${direction} bpf direct-action obj ${rtarg}.o sec tc

rebuild-all:
	tc filter del dev eth0 ingress
	tc filter del dev eth1 ingress
	tc filter del dev eth0 egress
	tc filter del dev eth1 egress
	clang -g -O2 -I/usr/include/aarch64-linux-gnu -Wall -target bpf -c ${rtarg}.bpf.c -o ${rtarg}.o
	tc filter add dev eth0 ingress bpf direct-action obj ${rtarg}.o sec tc
	tc filter add dev eth1 ingress bpf direct-action obj ${rtarg}.o sec tc
	tc filter add dev eth0 egress bpf direct-action obj ${rtarg}.o sec tc
	tc filter add dev eth1 egress bpf direct-action obj ${rtarg}.o sec tc

detach-all:
	tc filter del dev eth0 ingress
	tc filter del dev eth1 ingress
	tc filter del dev eth0 egress
	tc filter del dev eth1 egress


clean:
	# - bpftool net detach xdp dev lo
	# - bpftool net detach xdp dev docker0
	- sudo rm -f /sys/fs/bpf/$(TARGET)
	- rm $(BPF_OBJ)
	# - tc filter delete dev docker0 parent ffff:

