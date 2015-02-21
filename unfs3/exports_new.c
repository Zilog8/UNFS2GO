/*
 * UNFS3 exports parser and export controls
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */
#include <netdb.h>	//hostent
#include <arpa/inet.h> //inet_ntoa


/* export list, item, and host filled during parse */
static e_item *e_list = NULL;
static e_item cur_item;
static e_host cur_host;

/* last looked-up anonuid and anongid */
static uint32 last_anonuid = ANON_NOTSPECIAL;
static uint32 last_anongid = ANON_NOTSPECIAL;

/* mount protocol compatible variants */
static exports ne_list = NULL;
static struct exportnode ne_item;
static struct groupnode ne_host;

/* error status of last parse */
int e_error = FALSE;

/*
 * The FNV1a-32 hash algorithm
 * (http://www.isthe.com/chongo/tech/comp/fnv/)
 */
uint32 fnv1a_32(const char *str, uint32 hval)
{
    static const uint32 fnv_32_prime = 0x01000193;
    
    while (*str) {
	hval ^= *str++;
	hval *= fnv_32_prime;
    }
    return hval;
}

/*
 * get static fsid, for use with removable media export points
 */
static uint32 get_free_fsid(const char *path)
{
    uint32 hval;

    /* The 32:th bit is set to one on all special filehandles. The
       last 31 bits are hashed from the export point path. */
    hval = fnv1a_32(path, 0);
    hval |= 0x80000000;
    return hval;
}


/*
 * clear current host
 */
static void clear_host(void)
{
	memset(&cur_host, 0, sizeof(e_host));
	strcpy(cur_host.orig, "<anon clnt>");
	memset(&ne_host, 0, sizeof(struct groupnode));
	
	cur_host.anonuid =
	cur_host.anongid = ANON_NOTSPECIAL; 
}

/*
 * clear current item
 */
static void clear_item(void)
{
	memset(&cur_item, 0, sizeof(e_item));
	memset(&ne_item, 0, sizeof(struct exportnode));
}

/* 
 * add current host to current export item
 */
static void add_host(void)
{
	e_host *new;
	e_host *iter;
	
	groups ne_new;
	groups ne_iter;

	new = malloc(sizeof(e_host));
	ne_new = malloc(sizeof(struct groupnode));
	if (!new || !ne_new) {
		fprintf(stderr, "out of memory, aborting\n");
		daemon_exit(CRISIS);
	}

	*new = cur_host;
	*ne_new = ne_host;
	ne_new->gr_name = new->orig;

	/* internal list */
	cur_item.hosts = new;

	/* matching mount protocol list */
	ne_item.ex_groups = ne_new;
	
	clear_host();
}

/* 
   Normalize path, eliminating double slashes, etc. To be used instead
   of realpath, when realpath is not possible. Normalizing export
   points is important. Otherwise, mount requests might fail, since
   /x/y is not a prefix of ///x///y/ etc.
*/
char *normpath(const char *path, char *normpath)
{
	char *n;
	const char *p;

	/* Copy path to normpath, and replace blocks of slashes with
	   single slash */
	p = path;
	n = normpath;
	while (*p) {
		/* Skip over multiple slashes */
		if (*p == '/' && *(p+1) == '/') {
			p++;
			continue;
		}
		*n++ = *p++;
	}
	*n = '\0';

	/* Remove trailing slash, if any. */
	if ((n - normpath) > 1 && *(n-1) == '/')
		*(n-1) = '\0';

	return normpath;
}

/*
 * add current item to current export list
 */
static void add_item(const char *path)
{
	char buf[PATH_MAX];
	e_item *new;
	e_item *iter;
	e_host *host;
	/* Is this item marked as removable for all hosts? */
	int removable_for_all = 1;
	
	exports ne_new;
	exports ne_iter;

	new = malloc(sizeof(e_item));
	ne_new = malloc(sizeof(struct exportnode));
	if (!new || !ne_new) {
		fprintf(stderr, "out of memory, aborting\n");
		daemon_exit(CRISIS);
	}

	/* Loop over all hosts and check if marked as removable. */
	host = cur_item.hosts;
	if (!(host->options & OPT_REMOVABLE))
		removable_for_all = 0;

	if (removable_for_all) {
		/* If marked as removable for all hosts, don't try
		   realpath. */
		normpath(path, buf);
	} else if (!go_realpath(path, buf)) {
		fprintf(stderr, "realpath for %s failed\n", path);
		e_error = TRUE;
		free(new);
		free(ne_new);
		clear_item();
		return;
	}

	if (strlen(buf) + 1 > NFS_MAXPATHLEN) {
		fprintf(stderr, "attempted to export too long path\n");
		e_error = TRUE;
		free(new);
		free(ne_new);
		clear_item();
		return;
	}

	/* if no hosts listed, list default host */
	if (!cur_item.hosts)
		add_host();

	*new = cur_item;
	strcpy(new->path, buf);
	strcpy(new->orig, path);
	new->fsid = get_free_fsid(path);  
	new->last_mtime = 0;
	new->dir_hash = 0;

	*ne_new = ne_item;
	ne_new->ex_dir = new->orig;

	/* internal list */
	e_list = new;

	/* matching mount protocol list */
	ne_list = ne_new;
		
	clear_item();
}

/*
 * fill current host's address given a hostname
 */
static void set_hostname(const char *name)
{
	struct hostent *ent;

	if (strlen(name) + 1 > NFS_MAXPATHLEN) {
		e_error = TRUE;
		return;
	}
	strcpy(cur_host.orig, name);

	ent = gethostbyname(name);

	if (ent) {
		memcpy(&cur_host.addr, ent->h_addr_list[0],
		       sizeof(struct in_addr));
		cur_host.mask.s_addr = 0;
		cur_host.mask.s_addr = ~cur_host.mask.s_addr;
	} else {
		fprintf(stderr, "could not resolve hostname '%s'\n", name);
		e_error = TRUE;
	}
}	

/*
 * compute network bitmask
 */
static unsigned long make_netmask(int bits) {
	unsigned long buf = 0;
	int i;

	for (i=0; i<bits; i++)
		buf = (buf << 1) + 1;
	for (; i < 32; i++)
		buf = (buf << 1);
	return htonl(buf);
}

/*
 * add an option bit to the current host
 */
static void add_option(const char *opt)
{
	if (strcmp(opt,"no_root_squash") == 0)
		cur_host.options |= OPT_NO_ROOT_SQUASH;
	else if (strcmp(opt,"root_squash") == 0)
		cur_host.options &= ~OPT_NO_ROOT_SQUASH;
	else if (strcmp(opt,"all_squash") == 0)
		cur_host.options |= OPT_ALL_SQUASH;
	else if (strcmp(opt,"no_all_squash") == 0)
		cur_host.options &= ~OPT_ALL_SQUASH;
	else if (strcmp(opt,"rw") == 0)
		cur_host.options |= OPT_RW;
	else if (strcmp(opt,"ro") == 0)
		cur_host.options &= ~OPT_RW;
	else if (strcmp(opt,"removable") == 0) {
		cur_host.options |= OPT_REMOVABLE;
	} else if (strcmp(opt,"fixed") == 0)
		cur_host.options &= ~OPT_REMOVABLE;
	else if (strcmp(opt,"insecure") == 0)
		cur_host.options |= OPT_INSECURE;
	else if (strcmp(opt,"secure") == 0)
		cur_host.options &= ~OPT_INSECURE;
	else
		fprintf(stderr, "Warning: unknown exports option `%s' ignored\n",
			opt);
}

static void add_option_with_value(const char *opt, const char *val)
{
    if (strcmp(opt,"anonuid") == 0) {
    	cur_host.anonuid = atoi(val);
    } else if (strcmp(opt,"anongid") == 0) {
    	cur_host.anongid = atoi(val);
    } else {
        fprintf(stderr,  "Warning: unknown exports option `%s' ignored\n",
            opt);
    }
}

/* effective export list and access flag */
static e_item *export_list = NULL;
static volatile int exports_access = FALSE;

/* mount protocol compatible exports list */
exports exports_nfslist = NULL;

/*
 * print out the current exports list (for debugging)
 */
void print_list(void)
{
	char addrbuf[16], maskbuf[16];

	e_item *item;
	e_host *host;
	
	item = e_list;
	fprintf(stderr, "Print list\n");
	fprintf(stderr, "Item: path %s orig %s\n", item->path, item->orig);
	host = item->hosts;
	/* inet_ntoa returns static buffer */
	strcpy(addrbuf, inet_ntoa(host->addr));
	strcpy(maskbuf, inet_ntoa(host->mask));
	fprintf(stderr, "%s: ip %s mask %s options %i\n",
			item->path, addrbuf, maskbuf,
			host->options);
}

/*
 * clear current parse state
 */
static void clear_cur(void)
{
	e_list = NULL;
	ne_list = NULL;
	e_error = FALSE;
	clear_host();
	clear_item();
}

/*
 * parse an exports file
 */
int exports_parse(char *exportString, char *exportOpts)
{
	//TODO: this isn't right at all, but it'll do for now.
	
	add_option(exportOpts);
	add_host();
	add_item(exportString);
		
	print_list();
	
	export_list = e_list;
	exports_nfslist = ne_list;
	return TRUE;
}

/*
 * find a given host inside a host list, return options
 */
static e_host* find_host(struct in_addr remote, e_item *item)
{
	e_host *host;
	host = item->hosts;
	if ((remote.s_addr & host->mask.s_addr) == host->addr.s_addr) {
		return host;
	}
	return NULL;
}

/* options cache */
int exports_opts = -1;
const char *export_path = NULL; 

/*
 * given a path, return client's effective options
 */
int exports_options(const char *path)
{
	e_item *list;
	struct in_addr remote;
	unsigned int last_len = 0;
	
	exports_opts = -1;
	export_path = NULL;
	last_anonuid = ANON_NOTSPECIAL;
	last_anongid = ANON_NOTSPECIAL;

	/* check for client attempting to use invalid pathname */
	if (!path || strstr(path, "/../")) {
		fprintf(stderr, "exports_options: Failed first check\n");
		return exports_opts;
		}
	
	/* protect against SIGHUP reloading the list */
	exports_access = TRUE;
	
	list = export_list;
		/* if path makes sense */
		if (strlen(list->path) > last_len && strstr(path, list->path) == path) {
		    e_host* cur_host = list->hosts;
			
			if (cur_host) {
				exports_opts = cur_host->options;
				export_path = list->path;
				last_len = strlen(list->path);
				last_anonuid = cur_host->anonuid;
				last_anongid = cur_host->anongid;
			}
		}
		
	exports_access = FALSE;
	return exports_opts;
}

/*
 * check whether path is an export point
 */
int export_point(const char *path)
{
	exports_access = TRUE;

	if (strcmp(path, export_list->path) == 0) {
		exports_access = FALSE;
		return TRUE;
	}
	exports_access = FALSE;
	return FALSE;
}

/*
 * check whether options indicate rw mount
 */
nfsstat3 exports_rw(void)
{
	if (exports_opts != -1 && (exports_opts & OPT_RW))
		return NFS3_OK;
	else
		return NFS3ERR_ROFS;
}