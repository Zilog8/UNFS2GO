/*
 * UNFS3 server framework
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */

#ifndef UNFS3_DAEMON_H
#define UNFS3_DAEMON_H

#include "nfs.h"
#include "../backend_go.h"
#include "mount.h"
#include "fh.h"
#include "exports_new.h"
#include "xdr.h"
#include "attr.h"
#include "mount.h"

/* exit status for internal errors */
#define CRISIS	99

/* error handling */
void daemon_exit(int);

/* remote address */
struct in_addr get_remote(struct svc_req *);
int get_socket_type(struct svc_req *rqstp);

/* options */
extern int	opt_detach;
extern char	*opt_exports;
extern int	opt_cluster;
extern char	*opt_cluster_path;
extern int	opt_singleuser;
extern int	opt_brute_force;
extern int	opt_readable_executables;

#endif
