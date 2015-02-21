#ifndef UNFS3_BACKEND_GO_H
#define UNFS3_BACKEND_GO_H

#include "gosupport.h"
/*
 * system calls
 */
#define go_dirstream go_DIR
#define go_fsinfo_properties FSF3_LINK | FSF3_SYMLINK | FSF3_HOMOGENEOUS | FSF3_CANSETTIME;
#define go_time_delta_seconds 1
#define go_pathconf_case_insensitive FALSE
#endif
