
/*
 * UNFS3 low-level filehandle routines
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */

#if HAVE_LINUX_EXT2_FS_H == 1

/*
 * presence of linux/ext2_fs.h is a hint that we are on Linux, really
 * including that file doesn't work on Debian, so define the ioctl
 * number here
 */
#define EXT2_IOC_GETVERSION	0x80047601
#endif

/*
 * hash function for inode numbers
 */
#define FH_HASH(n) ((n ^ (n >> 8) ^ (n >> 16) ^ (n >> 24) ^ (n >> 32) ^ (n >> 40) ^ (n >> 48) ^ (n >> 56)) & 0xFF)

/*
 * --------------------------------
 * FILEHANDLE COMPOSITION FUNCTIONS
 * --------------------------------
 */

/*
 * check whether an NFS filehandle is valid
 */
int nfh_valid(nfs_fh3 fh)
{
    unfs3_fh_t *obj = (void *) fh.data.data_val;

    /* too small? */
    if (fh.data.data_len < FH_MINLEN)
	return FALSE;

    /* encoded length different from real length? */
    if (fh.data.data_len != fh_length(obj))
	return FALSE;

    return TRUE;
}

/*
 * check whether a filehandle is valid
 */
int fh_valid(unfs3_fh_t fh)
{
    /* invalid filehandles have zero device and inode */
    return (int) (fh.dev != 0 || fh.ino != 0);
}

/*
 * invalid fh for error returns
 */
#ifdef __GNUC__
static const unfs3_fh_t invalid_fh = {.dev = 0,.ino = 0,.gen = 0,.len =
	0,.inos = {0}
};
#else
static const unfs3_fh_t invalid_fh = { 0, 0, 0, 0, 0, {0} };
#endif

/*
 * compose a filehandle for a given path
 * path:     path to compose fh for
 * need_dir: if not 0, path must point to a directory
 */
unfs3_fh_t fh_comp_raw(const char *path, int need_dir)
{
    char work[NFS_MAXPATHLEN];
    unfs3_fh_t fh;
    backend_statstruct buf;
    int res;
    char *last;
    int pos = 0;

    fh.len = 0;

    res = backend_lstat(path, &buf);
	if (res == -2) {
		//fprintf(stderr, "fh_comp_raw: Not Found for '%s'\n", path);
	    return invalid_fh;
	}
    if (res == -1) {
	    //fprintf(stderr, "fh_comp_raw: failed second test for '%s'\n", path);
		return invalid_fh;
	}
    /* check for dir if need_dir is set */
    if (need_dir != 0 && !S_ISDIR(buf.st_mode)) {
		//fprintf(stderr, "fh_comp_raw: failed third test for '%s' mode was %i\n", path, buf.st_mode);
		return invalid_fh;
	}
	
    fh.dev = buf.st_dev;
    fh.ino = buf.st_ino;
    fh.gen = buf.st_ino;

    /* special case for root directory */
    if (strcmp(path, "/") == 0) {
		return fh;
	}
    strcpy(work, path);
    last = work;

    do {
	*last = '/';
	last = strchr(last + 1, '/');
	if (last != NULL)
	    *last = 0;

	res = backend_lstat(work, &buf);
	if (res == -2) {
		fprintf(stderr, "fh_comp_raw: Not Found for '%s'\n", path);
	    return invalid_fh;
	}
	if (res == -1) {
		fprintf(stderr, "fh_comp_raw: failed fourth test for '%s'\n", path);
	    return invalid_fh;
	}

	/* store 8 bit hash of the component's inode */
	fh.inos[pos] = FH_HASH(buf.st_ino);
	pos++;

    } while (last && pos < FH_MAXLEN);

    if (last) {			       /* path too deep for filehandle */
	   fprintf(stderr, "fh_comp_raw: failed fifth test for '%s'\n", path);
		return invalid_fh;
	}
    fh.len = pos;

    return fh;
}

/*
 * get real length of a filehandle
 */
u_int fh_length(const unfs3_fh_t * fh)
{
    return fh->len + sizeof(fh->len) + sizeof(fh->dev) + sizeof(fh->ino) +
	sizeof(fh->gen) + sizeof(fh->pwhash);
}

/*
 * extend a filehandle with a given device, inode, and generation number
 */
unfs3_fh_t *fh_extend(nfs_fh3 nfh, uint32 dev, uint64 ino)
{
    static unfs3_fh_t new;
    unfs3_fh_t *fh = (void *) nfh.data.data_val;

    memcpy(&new, fh, fh_length(fh));

    new.dev = dev;
    new.ino = ino;
    new.gen = ino;
    new.inos[new.len] = FH_HASH(ino);
    new.len++;

    return &new;
}

/*
 * get post_op_fh3 extended by device, inode, and generation number
 */
post_op_fh3 fh_extend_post(nfs_fh3 fh, uint32 dev, uint64 ino)
{
    post_op_fh3 post;
    unfs3_fh_t *new;

    new = fh_extend(fh, dev, ino);

    if (new) {
	post.handle_follows = TRUE;
	post.post_op_fh3_u.handle.data.data_len = fh_length(new);
	post.post_op_fh3_u.handle.data.data_val = (char *) new;
    } else
	post.handle_follows = FALSE;

    return post;
}

/*
 * extend a filehandle given a path and needed type
 */
post_op_fh3 fh_extend_type(nfs_fh3 fh, const char *path, unsigned int type)
{
    post_op_fh3 result;
    backend_statstruct buf;
    int res;

    res = backend_lstat(path, &buf);
    if (res < 0 || (buf.st_mode & type) != type) {
		return result;
    }

    return fh_extend_post(fh, buf.st_dev, buf.st_ino);
}

/*
 * -------------------------------
 * FILEHANDLE RESOLUTION FUNCTIONS
 * -------------------------------
 */

/*
 * filehandles have the following fields:
 * dev:  device of the file system object fh points to
 * ino:  inode of the file system object fh points to
 * gen:  inode generation number, if available
 * len:  number of entries in following inos array
 * inos: array of max FH_MAXLEN directories needed to traverse to reach
 *       object, for each name, an 8 bit hash of the inode number is stored
 *
 * - search functions traverse directory structure from the root looking
 *   for directories matching the inode information stored
 * - if such a directory is found, we descend into it trying to locate the
 *   object
 */

char *go_fgetpath(int);

/*
 * resolve a filehandle into a path
 */
char *fh_decomp_raw(const unfs3_fh_t * fh)
{
    char *rec;
    static char result[NFS_MAXPATHLEN];

    /* valid fh? */
    if (!fh)
	return NULL;

    /* special case for root directory */
    if (fh->len == 0)
	return "/";

	
    rec = go_fgetpath(fh->ino);

    if (rec)
	return rec;

    /* could not find object */
    return NULL;
}
