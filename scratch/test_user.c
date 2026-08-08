#include <stdio.h>
#include <stdlib.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/ioctl.h>

#define DEMO_IOCTL_MAGIC 'k'
#define DEMO_IOCTL_RESET _IO(DEMO_IOCTL_MAGIC, 1)

int main() {
    int fd = open("/dev/demo_dev", O_RDWR);
    if (fd < 0) {
        perror("Failed to open /dev/demo_dev");
        return 1;
    }

    char buf[64] = {0};
    read(fd, buf, sizeof(buf) - 1);
    printf("Read from kernel driver: %s", buf);

    printf("Sending DEMO_IOCTL_RESET to kernel...\n");
    ioctl(fd, DEMO_IOCTL_RESET, 0);

    read(fd, buf, sizeof(buf) - 1);
    printf("Read after reset: %s", buf);

    close(fd);
    return 0;
}
