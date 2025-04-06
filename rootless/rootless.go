package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// This is a rootless container implementation with NO limits on resources (kind of like running a regular process with no safeguards).
// The other implementation has cgroups set up when running/initializing the container (writes to /sys/fs/cgroups virtual fs), which is why we need sudo.
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
	// Here, it sets the process to run in a new Unix Timesharing System NAMESPACE | new PID NAMESPACE | new mount NAMESPACE | new USER (local root)
	// Also unshares any recursively inherited mount properties from host machine
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Cloneflags has logical OR to combine the different flags (used in C-style APIs)
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
		// Cannot see container's proc mount in host (see notes)
		Unshareflags: syscall.CLONE_NEWNS,
		// This is to make the container rootless by setting that we want root credentials in the container (act as uid 0 and gid 0)
		Credential: &syscall.Credential{Uid: 0, Gid: 0},
		// This is to map the current user id and group id as the new root user (current id -> container id FOR BOTH UID AND GID)
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
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
		os.Exit(1)
	}

	fmt.Printf("Container Wrapper has gracefully shutdown!\n")
}

func child() {
	fmt.Printf("Child running %v as %d\n", os.Args[2:], os.Getpid())

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

	// ANOTHER WAY OF DOING GRACEFUL SHUTDOWN (NOT THE MODERN WAY OF DOING IT)
	// Create a channel for graceful shutdown -> though this is UNNECESSARY since its a single threaded program
	// We can just use a simple if-statement instead
	done := make(chan error, 1)
	// Spawn a goroutine where cmd.Run() is a blocking call -> reinvoke same process with cmd.Run()
	go func() {
		done <- cmd.Run() // If this finishes, it will pass an error to the done channel
	}()

	// Create a channel to handle SIGINT and SIGTERM in main go program
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM) // relays SIGINT and SIGTERM to sigs channel

	// Wait for bash in container to exit or wait for SIGINT/SIGTERM in main program
	select {
	case err := <-done:
		fmt.Printf("Container exited: %v\n", err)
	case sig := <-sigs:
		fmt.Printf("SIGINT/SIGTERM received: %v\n", sig)
		if cmd.Process != nil {
			_ = cmd.Process.Signal(sig) // forward the signal to the bash command
		}
		<-done // wait for the container to actually exit (blocking)
	}

	// Cleanup
	// Unmount the /proc pseudo-filesystem: we can use "/proc" in the target field
	if err := syscall.Unmount("proc", 0); err != nil {
		log.Fatalf("/proc unmount failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Container and program has gracefully shutdown!\n")

	// Exit from child process and return to run()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
