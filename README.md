# GoContainer

A simple container (like Docker) built from scratch using Go. This project explores containerization concepts such as namespaces, cgroups, and filesystem isolation, offering a lightweight and educational alternative to full-fledged container platforms.

## 🚀 Features

- Process isolation using Linux namespaces
- Resource limits via cgroups
- File system isolation with `chroot`
- Custom image execution
- Basic CLI interface

## 🧠 Motivation

This project was built as a learning exercise to understand how container technologies like Docker work under the hood using core Linux primitives and Go's `syscall` package.

## ⚙️ Requirements

- Linux (required for namespaces and cgroups)
- Go 1.18+
- Root privileges (to set up namespaces and cgroups)

## 📦 Installation

```bash
git clone https://github.com/yourusername/gocontainer.git
cd gocontainer
go build -o gocontainer
```

## 🧪 Usage

```bash
sudo ./gocontainer run <command>
```

Example:

```bash
sudo ./gocontainer run /bin/bash
```

This will spin up an isolated shell environment with process and filesystem isolation.

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
```

## 📚 Concepts Used

- **Namespaces**: Isolates process trees, networking, hostnames, etc.
- **Cgroups**: Limits CPU/memory usage for containers.
- **Chroot/Pivot\_root**: Provides a root filesystem environment.
- **Syscalls**: Low-level OS control using Go's `syscall` and `unix` packages.

## 🙌 Acknowledgements

Inspired by:

- [Liz Rice's container from scratch](https://www.youtube.com/watch?v=8fi7uSYlOdc)
- [runc](https://github.com/opencontainers/runc)
- [Docker internals](https://github.com/moul/awesome-docker#internals)

## 📝 License

MIT License


