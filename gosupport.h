#ifndef UNFS3_GOSUPPORT_H
#define UNFS3_GOSUPPORT_H

#include "unfs3/nfs.h"

//Borrowed these from Linux/include/uapi/linux/stat.h
#define S_IFMT  00170000
#define S_IFSOCK 0140000
#define S_IFLNK  0120000
#define S_IFREG  0100000
#define S_IFBLK  0060000
#define S_IFDIR  0040000
#define S_IFCHR  0020000
#define S_IFIFO  0010000
#define S_ISUID  0004000
#define S_ISGID  0002000
#define S_ISVTX  0001000

#define S_ISLNK(m)      (((m) & S_IFMT) == S_IFLNK)
#define S_ISREG(m)      (((m) & S_IFMT) == S_IFREG)
#define S_ISDIR(m)      (((m) & S_IFMT) == S_IFDIR)
#define S_ISCHR(m)      (((m) & S_IFMT) == S_IFCHR)
#define S_ISBLK(m)      (((m) & S_IFMT) == S_IFBLK)
#define S_ISFIFO(m)     (((m) & S_IFMT) == S_IFIFO)
#define S_ISSOCK(m)     (((m) & S_IFMT) == S_IFSOCK)

#define S_IRWXU 00700
#define S_IRUSR 00400
#define S_IWUSR 00200
#define S_IXUSR 00100

#define S_IRWXG 00070
#define S_IRGRP 00040
#define S_IWGRP 00020
#define S_IXGRP 00010

#define S_IRWXO 00007
#define S_IROTH 00004
#define S_IWOTH 00002
#define S_IXOTH 00001


#define O_RDONLY      00
#define O_WRONLY      01
#define O_RDWR        02
#define O_CREAT       0100
#define O_EXCL		  0200
#define O_TRUNC       01000
#define O_NONBLOCK	  04000

/* Only includes fields actually used by unfs3 */
typedef struct _go_statvfsstruct
{
        unsigned long  f_frsize;    /* file system block size */
        uint64         f_blocks;   /* size of fs in f_frsize units */
        uint64         f_bfree;    /* # free blocks */
        uint64         f_bavail;   /* # free blocks for non-root */
        uint64         f_files;    /* # inodes */
        uint64         f_ffree;    /* # free inodes */
} go_statvfsstruct;

typedef struct _go_statstruct
{
        uint32  st_dev;  
        uint64  st_ino;
        short st_mode;
        short   st_nlink;
        uint32  st_uid;
        uint32  st_gid;
        uint32  st_rdev;
        uint64 st_size;
        short   st_blksize;
        uint32  st_blocks;
        time_t  st_atime;
        time_t  st_mtime;
        time_t  st_ctime;
} go_statstruct;
#endif
