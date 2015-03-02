
/*
 * UNFS3 low-level filehandle routines
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */

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
 * get real length of a filehandle
 */
u_int fh_length(const unfs3_fh_t * fh)
{
    return fh->len + sizeof(fh->len) + sizeof(fh->ino);
}

char *go_fgetpath(int inode);

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
	
	return go_fgetpath(obj->ino);
}

//Create new filehandle
unfs3_fh_t *fh_comp(uint64 ino, const char *path)
{
	static unfs3_fh_t new;
	new.ino = ino;
	new.len = 1;

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
		result.handle_follows = FALSE;
		return result;
    }

    return fh_comp_post(buf.st_ino, path);
}