
/*
 * UNFS3 mount protocol procedures
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */
#include <arpa/inet.h> //inet_ntoa

#ifndef PATH_MAX
# define PATH_MAX	4096
#endif

#define IS_SECURE(port) ((port) < 1024)

/*
 * number of active mounts
 *
 * only a guess since clients can crash and/or not sent UMNT calls
 */
static int mount_cnt = 0;

/* list of currently mounted directories */
static mountlist mount_list = NULL;

static char nonce[32] = "";

/*
 * add entry to mount list
 */
static void add_mount(const char *path, struct svc_req *rqstp)
{
    mountlist new;
    mountlist iter;
    char *host;

    new = malloc(sizeof(struct mountbody));
    if (!new) {
	fprintf(stderr, "%s\n", "add_mount: Unable to allocate memory");
	return;
    }

    host = inet_ntoa(get_remote(rqstp));
    new->ml_hostname = malloc(strlen(host) + 1);
    if (!new->ml_hostname) {
	fprintf(stderr, "%s\n", "add_mount: Unable to allocate memory");
	free(new);
	return;
    }

    new->ml_directory = malloc(strlen(path) + 1);
    if (!new->ml_directory) {
	fprintf(stderr, "%s\n", "add_mount: Unable to allocate memory");
	free(new->ml_hostname);
	free(new);
	return;
    }

    /* initialize the new entry */
    new->ml_next = NULL;
    strcpy(new->ml_hostname, host);
    strcpy(new->ml_directory, path);

    iter = mount_list;
    if (iter) {
	while (iter->ml_next)
	    iter = iter->ml_next;
	iter->ml_next = new;
    } else
	mount_list = new;

    mount_cnt++;
}

/*
 * remove entries from mount list
 */
static void remove_mount(const char *path, struct svc_req *rqstp)
{
    mountlist iter, next, prev = NULL;
    char *host;

    host = inet_ntoa(get_remote(rqstp));

    iter = mount_list;
    while (iter) {
	if (strcmp(iter->ml_hostname, host) == 0 &&
	    (!path || strcmp(iter->ml_directory, path) == 0)) {
	    if (prev)
		prev->ml_next = iter->ml_next;
	    else
		mount_list = iter->ml_next;

	    next = iter->ml_next;

	    free(iter->ml_hostname);
	    free(iter->ml_directory);
	    free(iter);

	    iter = next;

	    /* adjust mount count */
	    if (mount_cnt > 0)
		mount_cnt--;
	} else {
	    prev = iter;
	    iter = iter->ml_next;
	}
    }
}

void *mountproc_null_3_svc(U(void *argp), U(struct svc_req *rqstp))
{
    static void *result = NULL;

    return &result;
}

mountres3 *mountproc_mnt_3_svc(dirpath * argp, struct svc_req * rqstp)
{
    char buf[PATH_MAX];
    static unfs3_fh_t fh;
    static mountres3 result;
    static int auth = AUTH_UNIX;
    int authenticated = 1;
    char *password;

    /* We need to modify the *argp pointer. Make a copy. */
    char *dpath = *argp;

    /* error out if not version 3 */
    if (rqstp->rq_vers != 3) {
	fprintf(stderr,
	       "%s attempted mount with unsupported protocol version\n",
	       inet_ntoa(get_remote(rqstp)));
	result.fhs_status = MNT3ERR_INVAL;
	return &result;
    }

    if (!go_realpath(dpath, buf)) {
	/* the given path does not exist */
	fprintf(stderr, "Mount svc: Given path does not exist\n");
	result.fhs_status = MNT3ERR_NOENT;
	return &result;
    }

    if (strlen(buf) + 1 > NFS_MAXPATHLEN) {
	fprintf(stderr, "%s attempted to mount jumbo path\n",
	       inet_ntoa(get_remote(rqstp)));
	result.fhs_status = MNT3ERR_NAMETOOLONG;
	return &result;
    }

    if ((exports_options(buf) == -1) || !go_accept_mount((svc_getcaller(rqstp->rq_xprt))->sin_addr, buf)) {
		/* not exported to this host*/
	fprintf(stderr, "Mount svc: Not exported to this host at all\n");
	result.fhs_status = MNT3ERR_ACCES;
	return &result;
    }

    fh = fh_comp_raw(buf, FH_DIR);

    if (!fh_valid(fh)) {
	fprintf(stderr, "%s attempted to mount non-directory\n",
	       inet_ntoa(get_remote(rqstp)));
	result.fhs_status = MNT3ERR_NOTDIR;
	return &result;
    }

    add_mount(dpath, rqstp);

    result.fhs_status = MNT3_OK;
    result.mountres3_u.mountinfo.fhandle.fhandle3_len = fh_length(&fh);
    result.mountres3_u.mountinfo.fhandle.fhandle3_val = (char *) &fh;
    result.mountres3_u.mountinfo.auth_flavors.auth_flavors_len = 1;
    result.mountres3_u.mountinfo.auth_flavors.auth_flavors_val = &auth;
	
    return &result;
}

mountlist *mountproc_dump_3_svc(U(void *argp), U(struct svc_req *rqstp))
{
    return &mount_list;
}

void *mountproc_umnt_3_svc(dirpath * argp, struct svc_req *rqstp)
{
    /* RPC times out if we use a NULL pointer */
    static void *result = NULL;

    remove_mount(*argp, rqstp);
    return &result;
}

void *mountproc_umntall_3_svc(U(void *argp), struct svc_req *rqstp)
{
    /* RPC times out if we use a NULL pointer */
    static void *result = NULL;

    remove_mount(NULL, rqstp);
    return &result;
}

exports *mountproc_export_3_svc(U(void *argp), U(struct svc_req *rqstp))
{
    return &exports_nfslist;
}
