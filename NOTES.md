```bash
ps # gets the list of processes from /proc
```

1. /proc contains all of the information of running processes
    - Need to implement my own /proc directory in container by implementing my own root directory for the spawned in container -> in ~/Containers/go-container-ubuntufs
    - Created ubuntu file system using debootstrap
    ```bash
    sudo debootstrap --variant=minbase focal ./ubuntu-rootfs http://archive.ubuntu.com/ubuntu/
    ```
    - Can look at information about container process using 
    ```bash
    ls -l /proc/${container PID}
    ```