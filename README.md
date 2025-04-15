# GoContainer

A simple container (like Docker) built from scratch using Go. This project explores containerization concepts such as namespaces, cgroups, and filesystem isolation, offering a lightweight and educational alternative to full-fledged container platforms.

## 🚀 Features
- [x] Process isolation using Linux namespaces
    - [x] PID Namespace - isolates process tree
    - [x] NET Namespace - isolates network stack
    - [x] MNT Namspace - isolates filesystem mount points
    - [x] UTS Namespace - isolates hostname
    - [x] CGroups - isolates resources
- [x] Resource limits via cgroups
- [x] File system isolation with `chroot`
- [x] Rootless containers
- [x] Graceful shutdown

## 🗒️ Next Steps/TODO
- [ ] Implement these properties/namespaces
    - [ ] USER Namespace - isolates system user IDs
    - [ ] IPC - isolates inter-process communication utilities
- [ ] Custom image execution
- [ ] Improve namespace features
- [ ] Handle other signals/interrupts (develop more container runtime feats)
- [ ] Create simple container orchestration
    - [ ] Basic CLI interface

## 🧠 Motivation

This project was built as a learning exercise to understand how container technologies like Docker work under the hood using core Linux primitives and Go's `syscall` package.

## ⚙️ Requirements

- Linux (required for namespaces and cgroups)
- Go 1.18+
- Root privileges (to set up namespaces and cgroups)

## 📦 Installation

```bash
git clone https://github.com/yourusername/go-container.git
cd go-container
go build -o gocontainer
```

**Make sure to create a /home/${NAME}/Containers/go-container-ubuntufs directory.**
This project used debootstrap to create the ubuntufs
```bash
sudo debootstrap focal ./go-container-ubuntufs http://archive.ubuntu.com/ubuntu/
```

**If networking inside the container does not work, make sure the following are set/run**

### Packet Forwarding
```bash
sudo vim /etc/sysctl.conf 
# Then add "net.ipv4.ip_forward = 1"
```

### NAT masquerading 
```bash
iptables -t nat -A POSTROUTING -s 192.168.100.0/24 -o ${host-net-iface} -j MASQUERADE
# This is for your host machine to rewrite the container's IP to work with NAT
```

### IP Tables FORWARD Rules
```bash
sudo iptables -A FORWARD -i veth-host -o ${host-net-iface} -j ACCEPT
sudo iptables -A FORWARD -o veth-host -i ${host-net-iface} -m state --state RELATED,ESTABLISHED -j ACCEPT
# This allows for forwarding between veth and the host's net iface
```

Make it permanent using the following
```bash
sudo apt install iptables-persistent
# Run configuration commands above
sudo netfilter-persistent save
```

You can verify using
```bash
sudo iptables -t nat -L -v -n
sudo iptables -L -v -n
```

## 🧪 Usage

```bash
sudo ./gocontainer run <command>
# OR
./rootless_container run <command>
```

Example:

```bash
sudo ./gocontainer run /bin/bash
# OR
make run # runs the first line in example
# OR
./rootless_container run /bin/bash
```

This will spin up an isolated shell environment with process and filesystem isolation.


<!-- 
## 📁 Project Structure

```
.
├── main.go          # Entry point of the CLI
├── container/       # Logic for namespace and cgroup setup
│   ├── cgroups.go
│   ├── namespaces.go
│   └── filesystem.go
├── utils/           # Utility functions
├── go.mod
└── README.md
``` -->

<!-- ## 📚 Concepts Used

- **Namespaces**: Isolates process trees, networking, hostnames, etc.
- **Cgroups**: Limits CPU/memory usage for containers.
- **Chroot/Pivot\_root**: Provides a root filesystem environment.
- **Syscalls**: Low-level OS control using Go's `syscall` and `unix` packages. -->

## 🙌 Acknowledgements

Inspired by:

- [Liz Rice's container from scratch](https://www.youtube.com/watch?v=8fi7uSYlOdc)
- [runc](https://github.com/opencontainers/runc)
- [Docker](https://github.com/docker)

## 📝 License

MIT License


