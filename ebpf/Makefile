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

clean:
	# - bpftool net detach xdp dev lo
	# - bpftool net detach xdp dev docker0
	- sudo rm -f /sys/fs/bpf/$(TARGET)
	- rm $(BPF_OBJ)
	# - tc filter delete dev docker0 parent ffff:
