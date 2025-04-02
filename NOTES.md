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


## Docker Containers

1. Use Docker to test and see docker creating a control group for a container in the /sys/fs/cgroup/${SPECIFIC CGROUP} directory
    ```bash
    docker pull ubuntu
    docker run --rm -it --memory=100M ubuntu /bin/bash
    ```