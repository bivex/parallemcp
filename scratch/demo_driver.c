#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/fs.h>
#include <linux/cdev.h>
#include <linux/uaccess.h>

#define DEVICE_NAME "demo_dev"
#define CLASS_NAME  "demo_class"
#define DEMO_IOCTL_MAGIC 'k'
#define DEMO_IOCTL_RESET _IO(DEMO_IOCTL_MAGIC, 1)

static int major_number;
static struct class*  demo_class  = NULL;
static struct device* demo_device = NULL;
static int counter = 100;

static int     dev_open(struct inode *, struct file *);
static int     dev_release(struct inode *, struct file *);
static ssize_t dev_read(struct file *, char *, size_t, loff_t *);
static ssize_t dev_write(struct file *, const char *, size_t, loff_t *);
static long    dev_ioctl(struct file *, unsigned int, unsigned long);

static struct file_operations fops = {
   .open = dev_open,
   .read = dev_read,
   .write = dev_write,
   .release = dev_release,
   .unlocked_ioctl = dev_ioctl,
};

static int __init demo_init(void) {
   pr_info("demo_driver: Initializing Linux Kernel Driver\n");

   major_number = register_chrdev(0, DEVICE_NAME, &fops);
   if (major_number < 0) {
      pr_err("demo_driver failed to register a major number\n");
      return major_number;
   }

   demo_class = class_create(CLASS_NAME);
   if (IS_ERR(demo_class)) {
      unregister_chrdev(major_number, DEVICE_NAME);
      return PTR_ERR(demo_class);
   }

   demo_device = device_create(demo_class, NULL, MKDEV(major_number, 0), NULL, DEVICE_NAME);
   if (IS_ERR(demo_device)) {
      class_destroy(demo_class);
      unregister_chrdev(major_number, DEVICE_NAME);
      return PTR_ERR(demo_device);
   }

   pr_info("demo_driver: device created correctly under /dev/%s (major %d)\n", DEVICE_NAME, major_number);
   return 0;
}

static void __exit demo_exit(void) {
   device_destroy(demo_class, MKDEV(major_number, 0));
   class_unregister(demo_class);
   class_destroy(demo_class);
   unregister_chrdev(major_number, DEVICE_NAME);
   pr_info("demo_driver: Goodbye from Linux Kernel Driver!\n");
}

static int dev_open(struct inode *inodep, struct file *filep) {
   pr_info("demo_driver: Device opened\n");
   return 0;
}

static ssize_t dev_read(struct file *filep, char *buffer, size_t len, loff_t *offset) {
   char msg[64];
   int msg_len;
   int error_count;

   msg_len = snprintf(msg, sizeof(msg), "Kernel Counter: %d\n", counter++);
   if (*offset >= msg_len) return 0;

   error_count = copy_to_user(buffer, msg, msg_len);
   if (error_count == 0) {
      *offset += msg_len;
      return msg_len;
   } else {
      return -EFAULT;
   }
}

static ssize_t dev_write(struct file *filep, const char *buffer, size_t len, loff_t *offset) {
   pr_info("demo_driver: Received %zu bytes from user\n", len);
   return len;
}

static long dev_ioctl(struct file *filep, unsigned int cmd, unsigned long arg) {
   if (cmd == DEMO_IOCTL_RESET) {
      counter = 100;
      pr_info("demo_driver: Reset counter to 100 via ioctl\n");
      return 0;
   }
   return -EINVAL;
}

static int dev_release(struct inode *inodep, struct file *filep) {
   pr_info("demo_driver: Device closed\n");
   return 0;
}

module_init(demo_init);
module_exit(demo_exit);

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Antigravity Engineer");
MODULE_DESCRIPTION("Complete Linux Kernel Character Driver for GDB Debugging");
MODULE_VERSION("1.0");
