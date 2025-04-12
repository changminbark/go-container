## Go Containers

1. Namespaces are what you can see and are created with syscalls
    - Unix Timesharing System is the hostname
    - Process IDs is the /proc mount
    - Network
    - User IDs
    - IPC
2. /proc contains all of the information of running processes
    - Need to implement my own /proc directory in container by implementing my own root directory for the spawned in container -> in ~/Containers/go-container-ubuntufs
    - Created ubuntu file system using debootstrap
    ```bash
    sudo debootstrap --variant=minbase focal ./ubuntu-rootfs http://archive.ubuntu.com/ubuntu/
    ```
    - Can look at information about container process using 
    ```bash
    ps # gets the list of processes from /proc
    ```
    ```bash
    ls -l /proc/${container PID}
    ```
3. /proc is a pseudo-filesystem, a mechanism for the kernel and userspace to share information.
    - Need to mount the /proc in the container as a pseudofile system.
    - Can check this on host terminal
    ```bash
    mount | grep proc
    ```
    - By default, under systemd, mounts recursively share properties, so root directory on host recurisvely share properties on any mounts. This is disabled with unshareflags.
    - Even with the unshareflags, we can still see the mount using
    ```bash
    cat /proc/${PID}/mounts
    ```
    - We can still see the processes from the host machine
    ```bash
    ps
    ```
4. Control Groups (CGroups) limit the resources of container with pseudo-filesystem interfaces
    - Memory
    - CPU
    - I/O
    - Process numbers
5. We can find the types of different cgroups we can set up
    ```bash
    ls /sys/fs/cgroup
    ```
    - We can look at the parameters of a cgroup too
    ```bash
    ls /sys/fs/cgroup/${SPECIFIC CGROUP}
    ```
    - For example, we can see limit of memory in bytes
    ```bash
    cat memory.limit_in_bytes
    ```
    - We may also find a docker subdirectory in each of these cgroup parameters
6. Similar to Docker, the Go Containers have the specific cgroup parameters set in each of the ${GO CONTAINER} subdirectory in a ${SPECIFIC CGROUP} directory.
7. We use the underlying clone() syscall instead of fork() as clone() gives more control over the resources that are shared between parent and child, such as memory space, file system, file descriptors, signal handlers, thread groups (used to create threads), and namespaces (using CLONE_NEW flags).
8. Rootless containers are containers that have the entire container runtime as well as the containers themselves not have access to root privileges. 
    - The container's view of the user id will be different from the host's view of the user id. 
    - Running
    ```bash
    ps
    ```
    as the root user will show the rootless container being run by a user that is not root (such as chang-min) even though the user inside the rootless container has the user name/id of root. Running the regular gocontainer will show that the container is being run by root.
9. Network namespaces also isolate the network resources (connections, network interfaces like host's eth0, routes, DNS, IP addresses, etc.)
    - We can check for the new network namespace by using
    ```bash
    cat /proc/self/net/tcp
    ```
    in the container
    - We can also use
    ```bash
    ls -l /proc/${PID}/ns/net
    ```
    from the host/parent process.
10. When you use CLONE_NEWPID, the PID namespace is only fully active for child processes of the process you created with clone().
    - So if you start a process with CLONE_NEWPID, that process becomes PID 1 in the new namespace — but doesn’t itself experience full PID isolation behavior. We can still see the process on the host machine.
    - That’s why you also need to spawn another child from within that process. That child will have:
        - PID ≠ 1 in the new namespace
        - Full namespace isolation (including network)



## Docker Containers

1. Use Docker to test and see docker creating a control group for a container in the /sys/fs/cgroup/${SPECIFIC CGROUP} directory
    ```bash
    docker pull ubuntu
    docker run --rm -it --memory=100M ubuntu /bin/bash
    ```