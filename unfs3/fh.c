
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
    return (int) (fh.ino != 0);
}

/*
 * invalid fh for error returns
 */
#ifdef __GNUC__
static const unfs3_fh_t invalid_fh = {.ino = 0,.len = 0,.path = {0} };
#else
static const unfs3_fh_t invalid_fh = { 0, 0, {0} };
#endif

/*
 * get real length of a filehandle
 */
u_int fh_length(const unfs3_fh_t * fh)
{
    return fh->len + sizeof(fh->len) + sizeof(fh->ino);
}

/*
 * resolve a filehandle into a path
 */
char *fh_decomp(nfs_fh3 fh)
{
    if (!nfh_valid(fh)) {
		return NULL;
    }

    unfs3_fh_t *obj = (void *) fh.data.data_val;

	if (obj->len == 0)   //root
		return "/";
	
	if (obj->len <= 33)  //small path <=32 bytes, plus add 1 for null-termination
		return obj->path;
	
	//long path, look it up
	return go_fgetpath(obj->ino);
}
	
//Create new filehandle
unfs3_fh_t *fh_comp(uint64 ino, const char *path)
{
	static unfs3_fh_t new;
	new.ino = ino;
	if (strlen(path)<=32) {      //small path <=32 bytes
	strcpy(new.path,path);
    new.len = (unsigned)strlen(new.path) + 1;
	} else {                    //long path, don't even bother to add
		new.len = 34;           //doesn't matter the length as long as it's greater than 33 (32 + null)
		strcpy(new.path,"");
	}

    return &new;
}

/*
 * get post_op_fh3 extended by device, inode, and path
 */
post_op_fh3 fh_comp_post(uint64 ino, const char *path)
{
    post_op_fh3 post;
    unfs3_fh_t *new;

    new = fh_comp(ino, path);

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
post_op_fh3 fh_comp_type(const char *path, unsigned int type)
{
    post_op_fh3 result;
    go_statstruct buf;

    if (go_lstat(path, &buf) != NFS3_OK || (buf.st_mode & type) != type) {
		return result;
    }

    return fh_comp_post(buf.st_ino, path);
}