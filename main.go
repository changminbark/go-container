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
	case "child":
		child()
	default:
		panic("Invalid command")
	}
}

func run() {
	fmt.Printf("Running %v as %d\n", os.Args[2:], os.Getpid())

	// Uses syscalls to execute external commands like execv
	// Creates structure of command
	// This creates a new *exec.Cmd object that re-executes the current binary
	// "/proc/self/exe" refers to currently running executable and the second part is providing arguments for the new process (manually simulating fork and exec)
	// “Run me again, but pass the argument "child" followed by the remaining original args starting from index 2 (after main.go + run).”
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Sets OS specific attributes.
	// Here, it sets the process to run in a new Unix Timesharing System NAMESPACE | new PID NAMESPACE | new mount NAMESPACE
	// Also unshares any recursively inherited mount properties from host machine
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Cloneflags has logical OR to combine the different flags (used in C-style APIs)
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		// Cannot see container's proc mount in host (see notes)
		Unshareflags: syscall.CLONE_NEWNS,
	}

	// Spawn a separate goroutine that listens for signals (SIGINT/SIGTERM) to pass to child() process
	// This ensures killing the main process (main()) will also force the child() to gracefully shutdown
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		for sig := range sigs { // Blocking loop (in case someone spams ctrl+c)
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	// Reinvoke same process in a new namespace (will run child() now)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running command: %v\n", err)
	}

	fmt.Printf("Container Wrapper has gracefully shutdown!\n")
}

func child() {
	// Set up context for receiving SIGINT and SIGTERM for "/proc/self/exe child /bin/bash"
	// When this process receives a SIGINT/SIGTERM, it will pass SIGKILL to the bash command
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Child running %v as %d\n", os.Args[2:], os.Getpid())

	if isCgroupV2() {
		cg2()
	} else {
		cg1()
	}

	// Cleanup for control group creation
	defer func() {
		err := os.RemoveAll("/sys/fs/cgroup/gocontainer/")
		if err != nil {
			log.Printf("Warning: failed to clean up cgroup directory: %v\n", err)
		}
	}()

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

	// This creates a command that will automatically terminate with a notify ctx to the goroutine calling child()
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
		log.Fatalf("/proc unmount failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Container has gracefully shutdown!\n")

	// Runs stop() and then exit from child process/return to main process so run() can terminate
}

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

func must(err error, msg ...string) {
	if err != nil {
		if len(msg) > 0 {
			log.Fatalf("%s: %v\n", msg[0], err)
		} else {
			log.Fatalf("Fatal error: %v\n", err)
		}
	}
}
