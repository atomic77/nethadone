## Development and Contribution

If you are interested in making changes to Nethadone, the least
bad option is to use a local virtual machine. Unfortunately 
containers are not sufficient as we need a fully emulated
Linux kernel running a specific version.

The easiest way to do this is, ironically, use the Dockerfile
to build a docker image, then use 
[`d2vm`](https://github.com/linka-cloud/d2vm) 
to create a VM based on your image:

```bash
docker buildx build -t atomic77/nethadone:latest .
```

Then use `d2vm` to create a qcow file which can then be used with full kernel virtualization:

```bash
sudo d2vm convert atomic77/nethadone -o nethadone.qcow2 -p 1234
d2vm run qemu --networking bridge,virbr0 --mem 4096 --cpus 4 ../nethadone.qcow2 
```

If using VMWare on Windows, you can also convert the qcow:

```bash
qemu-img convert nethadone.qcow2 -O vmdk nethadone.vmdk
```

Then import the VMDK using the GUI tools. 

I have not had much luck with VirtualBox. 
