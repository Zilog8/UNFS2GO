#ifndef UNFS3_BACKEND_GO_H
#define UNFS3_BACKEND_GO_H

#include "gosupport.h"
/*
 * backend init and shutdown
 */
#define backend_init go_init
#define backend_shutdown go_shutdown

/*
 * unfs3 functions
 */
#define backend_get_gen get_gen
#define backend_mksocket go_mksocket
#define backend_locate_file go_locate_file
#define backend_gen_nonce gen_nonce

/*
 * system calls
 */
#define backend_chmod go_chmod
#define backend_chown go_chown
#define backend_lchown go_lchown
#define backend_close go_close
#define backend_closedir go_closedir
#define backend_fchmod go_fchmod
#define backend_fchown go_fchown
#define backend_fstat go_fstat
#define backend_fsync go_fsync
#define backend_ftruncate go_ftruncate
#define backend_getegid getegid
#define backend_geteuid geteuid
#define backend_getgid getgid
#define backend_getuid getuid
#define backend_link go_link
#define backend_lseek go_lseek
#define backend_lstat go_lstat
#define backend_mkdir go_mkdir
#define backend_mkfifo go_mkfifo
#define backend_mknod go_mknod
#define backend_open go_open
#define backend_open_create go_open_create
#define backend_opendir go_opendir
#define backend_pread go_pread
#define backend_pwrite go_pwrite
#define backend_readdir go_readdir
#define backend_readlink go_readlink
#define backend_realpath go_realpath
#define backend_remove go_remove
#define backend_rename go_rename
#define backend_rmdir go_rmdir
#define backend_setegid setegid
#define backend_seteuid seteuid
#define backend_setgroups setgroups
#define backend_stat go_stat
#define backend_statvfs go_statvfs
#define backend_symlink go_symlink
#define backend_truncate go_truncate
#define backend_utime go_utime
#define backend_statstruct go_statstruct
#define backend_dirstream go_DIR
#define backend_statvfsstruct go_statvfsstruct
#define backend_fsinfo_properties FSF3_LINK | FSF3_SYMLINK | FSF3_HOMOGENEOUS | FSF3_CANSETTIME;
#define backend_time_delta_seconds 1
#define backend_pathconf_case_insensitive FALSE
#define backend_passwdstruct go_passwdstruct
#define backend_getpwnam(name) NULL
#define backend_flock go_flock
#define backend_getpid go_getpid
#define backend_store_create_verifier go_store_create_verifier
#define backend_check_create_verifier go_check_create_verifier
#endif
