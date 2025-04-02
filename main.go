package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func main() {
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

	// Reinvoke same process in a new namespace (will run child() now)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running command: %v\n", err)
		os.Exit(1)
	}
}

func child() {
	fmt.Printf("Child running %v as %d\n", os.Args[2:], os.Getpid())

	cg()

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

	// Need to unpack os.Args[3:], which is a slice, into variadic string parameters
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Reinvoke same process in a new namespace (created in run())
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error child running command: %v\n", err)
		os.Exit(1)
	}

	// Cleanup
	// We can use "/proc" in the target field
	if err := syscall.Unmount("proc", 0); err != nil {
		log.Fatalf("/proc unmount failed: %v\n", err)
		os.Exit(1)
	}
}

func cg() {
	cgroups := "/sys/fs/cgroup/"
	pids := filepath.Join(cgroups, "pids")
	// Create a control group called cm
	err := os.Mkdir(filepath.Join(pids, "cm"), 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}
	// Limit number of processes in cgroup to 20 processes (limits resources)
	must(os.WriteFile(filepath.Join(pids, "cm/pids.max"), []byte("20"), 0700))
	// This removes the new cgroup in place after the container exits
	must(os.WriteFile(filepath.Join(pids, "cm/notify_on_release"), []byte("1"), 0700))
	// This adds the current process into the cm cgroup
	must(os.WriteFile(filepath.Join(pids, "cm/cgroups.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
