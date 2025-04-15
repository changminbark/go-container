package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s run <cmd> [args...]", os.Args[0])
	}

	switch os.Args[1] {
	case "run":
		run()
	case "bootstrap":
		bootstrap()
	case "container":
		container()
	default:
		panic("Invalid command")
	}
}

// Function that runs the wrapper for the container (sets up namespaces for CLONE)
func run() {
	fmt.Printf("Running %v as %d\n", os.Args[2:], os.Getpid())

	// Create veth pair (veth-host & veth-cont)
	exec.Command("ip", "link", "add", "veth-host", "type", "veth", "peer", "name", "veth-cont").Run()
	// Assign IP on host to veth-host interface
	exec.Command("ip", "addr", "add", "192.168.100.1/24", "dev", "veth-host").Run()
	// Bring the veth-host interface up
	exec.Command("ip", "link", "set", "veth-host", "up").Run()

	// Uses syscalls to execute external commands like execv
	// Creates structure of command
	// This creates a new *exec.Cmd object that re-executes the current binary
	// "/proc/self/exe" refers to currently running executable and the second part is providing arguments for the new process (manually simulating fork and exec)
	// “Run me again, but pass the argument "bootstrap" followed by the remaining original args starting from index 2 (after main.go + run).”
	cmd := exec.Command("/proc/self/exe", append([]string{"bootstrap"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Sets OS specific attributes.
	// Here, it sets the process to run in a new Unix Timesharing System NAMESPACE | new PID NAMESPACE | new mount NAMESPACE | new network NAMESPACE
	// Also unshares any recursively inherited mount properties from host machine
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Cloneflags has logical OR to combine the different flags (used in C-style APIs)
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
		// Cannot see container's proc mount and network namespaces in host (see notes)
		Unshareflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	// This ensures killing the main process (main()) will also force the container() process to gracefully shutdown
	passSignal(cmd)

	// Reinvoke same process in a new namespace (will run container() now)
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start container environment: %v", err)
	}

	// Print the PID from the host's perspective
	fmt.Printf("Container bootstrap PID on host machine: %d\n", cmd.Process.Pid)

	// Move veth-cont link into container netns (isolates veth-cont from host, so it cannot be found on "ip link" on host)
	exec.Command("ip", "link", "set", "veth-cont", "netns", strconv.Itoa(cmd.Process.Pid)).Run()

	// Wait after printing
	if err := cmd.Wait(); err != nil {
		log.Printf("Container process exited: %v", err)
	}

	// Delete the veth created (cleanup)
	exec.Command("ip", "link", "del", "veth-host").Run()

	fmt.Printf("Container Wrapper has gracefully shutdown!\n")
}

// Bootstrapper function that acts as the root process in the container (namespaces all work properly for children processes)
func bootstrap() {
	// Run the command as a new process in the container namespace
	if os.Getpid() == 1 {
		fmt.Printf("We are PID 1 in new PID namespace - now forking actual container process\n")
		cmd := exec.Command("/proc/self/exe", append([]string{"container"}, os.Args[2:]...)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Ensure that SIGINT and SIGTERM signals are passed to container() for graceful shutdown
		passSignal(cmd)

		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to exec container: %v", err)
		}
	}
}

// Function that actually runs the container command as a child process (namespaces set up properly)
func container() {
	// Set up context for receiving SIGINT and SIGTERM for "/proc/self/exe container /bin/bash"
	// When this process receives a SIGINT/SIGTERM, it will pass SIGKILL to the bash command
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer stop()

	fmt.Printf("Container process running %v as %d\n", os.Args[2:], os.Getpid())

	// Create cgroup pseudo-filesystem
	if isCgroupV2() {
		cg2()
	} else {
		cg1()
	}

	newRoot := "/home/chang-min/Containers/go-container-ubuntufs"

	// Set the hostname of the new namespace
	if err := syscall.Sethostname([]byte("container")); err != nil {
		log.Fatalf("Sethostname failed: %v\n", err)
		os.Exit(1)
	}
	// Set the root of the process namespace to ROOT_FOR_GOCONTAINER
	if err := syscall.Chroot(newRoot); err != nil {
		log.Fatalf("Chroot failed: %v\n", err)
		os.Exit(1)
	}
	// Change the working directory to "/" (need this to set the working directory to the new root)
	if err := syscall.Chdir("/"); err != nil {
		log.Fatalf("Chroot failed: %v\n", err)
		os.Exit(1)
	}
	// Mount the /proc pseudo-filesystem (use "proc" instead of "/proc" since in a new root)
	// However, we can use "/proc" in the target field
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		log.Fatalf("/proc mount failed: %v\n", err)
		os.Exit(1)
	}

	// Print out network namespace
	link, _ := os.Readlink("/proc/self/ns/net")
	fmt.Printf("Network namespace:%s\n", link)

	// Sets loopback interface to up so that programs can use localhost
	exec.Command("ip", "link", "set", "lo", "up").Run()
	out, _ := exec.Command("ip", "addr").CombinedOutput()
	fmt.Printf("ip addr CMD:\n%s\n", string(out))

	// Rename veth-cont link/interface to eth0 (conventional name for primary network interface)
	exec.Command("ip", "link", "set", "veth-cont", "name", "eth0").Run()
	// Assign IP to eth0 (with local subnet 192.168.100.0 - 192.168.100.255 including gateway & broadcast)
	// Any IP outside this is not considered local and needs to access a router -> container can communicate with host without router
	exec.Command("ip", "addr", "add", "192.168.100.2/24", "dev", "eth0").Run()
	// Bring the eth0 interface up
	exec.Command("ip", "link", "set", "eth0", "up").Run()
	// Sets the default gateway (redirects packets to the given IP if not in routing table) to host
	exec.Command("ip", "route", "add", "default", "via", "192.168.100.1").Run()

	// This creates a command that will automatically terminate with a notify ctx to the goroutine calling container()
	cmd := exec.CommandContext(ctx, os.Args[2], os.Args[3:]...) // Need to unpack os.Args[3:], which is a slice, into variadic string parameters
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Starts and waits for command to finish
	if err := cmd.Run(); err != nil {
		fmt.Printf("Container exited with error: %v\n", err)
	} else {
		fmt.Printf("Container exited without error\n")
	}

	// Check error message for ctx and see if triggered by signal or not
	if ctx.Err() == context.Canceled {
		fmt.Println("Container shutdown triggered by signal")
	}

	// Cleanup
	// Unmount the /proc pseudo-filesystem: we can use "/proc" in the target field
	if err := syscall.Unmount("proc", 0); err != nil {
		log.Printf("/proc unmount failed: %v\n", err)
		os.Exit(1)
	}
	// Delete/unmount the /cgroup pseudo-filesystem for the container
	if err := os.RemoveAll("/sys/fs/cgroup/gocontainer/"); err != nil {
		log.Printf("Warning: failed to clean up cgroup directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Container has gracefully shutdown!\n")

	// Runs stop() and then exit from container process/return to main process so run() can terminate
}

// Function that checks for the version of Cgroup
func isCgroupV2() bool {
	statfs := syscall.Statfs_t{}
	err := syscall.Statfs("/sys/fs/cgroup", &statfs)
	if err != nil {
		return false
	}
	return statfs.Type == 0x63677270 // CGROUP2_SUPER_MAGIC
}

// https://docs.kernel.org/admin-guide/cgroup-v2.html
func cg1() {
	cgroups := "/sys/fs/cgroup/"
	pids := filepath.Join(cgroups, "pid/")
	// Create a pids control group called gocontainer
	err := os.MkdirAll(filepath.Join(pids, "gocontainer"), 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}
	// Limit number of processes in cgroup to 20 processes (limits resources)
	must(os.WriteFile(filepath.Join(pids, "gocontainer/pids.max"), []byte("20"), 0700), "Failed to write to cgroup")
	// This removes the new cgroup in place after the container exits
	must(os.WriteFile(filepath.Join(pids, "gocontainer/notify_on_release"), []byte("1"), 0700), "Failed to write to cgroup")
	// This adds the current process into the gocontainer cgroup
	must(os.WriteFile(filepath.Join(pids, "gocontainer/cgroups.procs"), []byte(strconv.Itoa(os.Getpid())), 0700), "Failed to write to cgroup")
}

// The cgroup filesystem (cgroup2fs) is a virtual filesystem managed by the Linux kernel.
// Creating a directory here will make the kernel automatically exposes resource control files for
// that cgroup based on the controllers available and enabled.
func cg2() {
	cgroups := "/sys/fs/cgroup/"
	containerCGroup := filepath.Join(cgroups, "gocontainer/")
	// Create a control group called gocontainer
	err := os.MkdirAll(containerCGroup, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}
	// Limit number of processes in cgroup to 20 processes (limits resources)
	must(os.WriteFile(filepath.Join(containerCGroup, "pids.max"), []byte("20"), 0700), "Failed to write to cgroup")
	// This adds the current process into the container cgroup
	must(os.WriteFile(filepath.Join(containerCGroup, "cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700), "Failed to write to cgroup")
}

// Function that spawns a goroutine that listens (blocking) for signals (SIGINT/SIGTERM) to pass to cmd() process
func passSignal(cmd *exec.Cmd) {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		for sig := range sigs { // Blocking loop (in case someone spams ctrl+c)
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()
}

func must(err error, msg ...string) {
	if err != nil {
		if len(msg) > 0 {
			log.Fatalf("%s: %v\n", msg[0], err)
		} else {
			log.Fatalf("Fatal error: %v\n", err)
		}
	}
}
