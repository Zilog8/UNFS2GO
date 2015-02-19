/*
 * UNFS3 export controls
 * (C) 2003, Pascal Schmidt
 * see file LICENSE for license details
 */

#ifndef UNFS3_EXPORTS_NEW_H
#define UNFS3_EXPORTS_NEW_H

#define OPT_NO_ROOT_SQUASH	1
#define OPT_ALL_SQUASH		2
#define OPT_RW			4
#define OPT_REMOVABLE		8
#define OPT_INSECURE		16

#define PASSWORD_MAXLEN   64

#define ANON_NOTSPECIAL 0xffffffff

typedef struct {
	char		orig[NFS_MAXPATHLEN];
	int		options;
        char            password[PASSWORD_MAXLEN+1];
        uint32          password_hash;
	struct in_addr	addr;
	struct in_addr	mask;
	uint32		anonuid;
	uint32		anongid;
	struct e_host	*next;
} e_host;

typedef struct {
	char		path[NFS_MAXPATHLEN];
	char		orig[NFS_MAXPATHLEN];
	e_host		*hosts;
	uint32          fsid; /* export point fsid (for removables) */
	time_t          last_mtime; /* Last returned mtime (for removables) */
	uint32          dir_hash; /* Hash of dir contents (for removables) */
	struct e_item	*next;
} e_item;

extern exports	exports_nfslist;
/* Options cache */
extern int	exports_opts;
extern const char *export_path; 
extern uint32 	export_fsid;
extern uint32   export_password_hash;

extern unsigned char password[PASSWORD_MAXLEN+1];

int exports_parse(char *exportString, char *exportOpts);
int		exports_options(const char *path, struct svc_req *rqstp, char **password, uint32 *fsid);
int             export_point(const char *path);
char            *export_point_from_fsid(uint32 fsid, time_t **last_mtime, uint32 **dir_hash);
nfsstat3	exports_compat(const char *path, struct svc_req *rqstp);
nfsstat3	exports_rw(void);
uint32          fnv1a_32(const char *str, uint32 hval);
char            *normpath(const char *path, char *normpath);

#endif
