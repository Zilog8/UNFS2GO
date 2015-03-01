
/*
 * UNFS3 attribute handling
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */

/*
 * find stat bit corresponding to given NFS file type
 */
mode_t type_to_mode(ftype3 ftype)
{
    switch (ftype) {
	case NF3REG:
	    return S_IFREG;
	case NF3DIR:
	    return S_IFDIR;
	case NF3LNK:
	    return S_IFLNK;
	case NF3CHR:
	    return S_IFCHR;
	case NF3BLK:
	    return S_IFBLK;
	case NF3FIFO:
	    return S_IFIFO;
	case NF3SOCK:
	    return S_IFSOCK;
    }

    /* fix gcc warning */
    return 0;
}

/*
 * post_op_attr for error returns
 */
#ifdef __GNUC__
static post_op_attr error_attr = {.attributes_follow = FALSE };
#else
static post_op_attr error_attr = { FALSE };
#endif

/*
 * return post-operation attributes
 */
post_op_attr get_post(const char *path, struct svc_req * req)
{
	go_statstruct buf;
    if (go_lstat(path, &buf) != NFS3_OK) {
		return error_attr;
    }
	
    return get_post_buf(buf, req);
}

post_op_attr get_post_err()
{
	return error_attr;
}

/*
 * return pre-operation attributes
 */
pre_op_attr get_pre_buf(go_statstruct buf)
{
    pre_op_attr result;
    result.attributes_follow = TRUE;

    result.pre_op_attr_u.attributes.size = buf.st_size;
    result.pre_op_attr_u.attributes.mtime.seconds = buf.st_mtime;
    result.pre_op_attr_u.attributes.mtime.nseconds = 0;
    result.pre_op_attr_u.attributes.ctime.seconds = buf.st_ctime;
    result.pre_op_attr_u.attributes.ctime.nseconds = 0;

    return result;
}

/*
 * return pre-operation attributes
 */
pre_op_attr get_pre(const char *path)
{
    pre_op_attr result;
	go_statstruct buf;
	
    if (go_lstat(path, &buf) != NFS3_OK) {
	result.attributes_follow = FALSE;
	return result;
    }

    result.attributes_follow = TRUE;

    result.pre_op_attr_u.attributes.size = buf.st_size;
    result.pre_op_attr_u.attributes.mtime.seconds = buf.st_mtime;
    result.pre_op_attr_u.attributes.mtime.nseconds = 0;
    result.pre_op_attr_u.attributes.ctime.seconds = buf.st_ctime;
    result.pre_op_attr_u.attributes.ctime.nseconds = 0;

    return result;
}

/*
 * compute post-operation attributes given a stat buffer
 */
post_op_attr get_post_buf(go_statstruct buf, struct svc_req * req)
{
    post_op_attr result;

    result.attributes_follow = TRUE;

    if (S_ISDIR(buf.st_mode))
	result.post_op_attr_u.attributes.type = NF3DIR;
    else if (S_ISBLK(buf.st_mode))
	result.post_op_attr_u.attributes.type = NF3BLK;
    else if (S_ISCHR(buf.st_mode))
	result.post_op_attr_u.attributes.type = NF3CHR;
#ifdef S_ISLNK
    else if (S_ISLNK(buf.st_mode))
	result.post_op_attr_u.attributes.type = NF3LNK;
#endif				       /* S_ISLNK */
#ifdef S_ISSOCK
    else if (S_ISSOCK(buf.st_mode))
	result.post_op_attr_u.attributes.type = NF3SOCK;
#endif				       /* S_ISSOCK */
    else if (S_ISFIFO(buf.st_mode))
	result.post_op_attr_u.attributes.type = NF3FIFO;
    else
	result.post_op_attr_u.attributes.type = NF3REG;

    /* adapt permissions for executable files */
    if (opt_readable_executables && S_ISREG(buf.st_mode)) {
	if (buf.st_mode & S_IXUSR)
	    buf.st_mode |= S_IRUSR;
	if (buf.st_mode & S_IXGRP)
	    buf.st_mode |= S_IRGRP;
	if (buf.st_mode & S_IXOTH)
	    buf.st_mode |= S_IROTH;
    }

    result.post_op_attr_u.attributes.mode = buf.st_mode & 0xFFFF;
    result.post_op_attr_u.attributes.nlink = buf.st_nlink;

    /* Normal case */
	result.post_op_attr_u.attributes.uid = buf.st_uid;
	result.post_op_attr_u.attributes.gid = buf.st_gid;

    result.post_op_attr_u.attributes.size = buf.st_size;
    result.post_op_attr_u.attributes.used = buf.st_blocks * 512;
    result.post_op_attr_u.attributes.rdev.specdata1 = (buf.st_rdev >> 8) & 0xFF;
    result.post_op_attr_u.attributes.rdev.specdata2 = buf.st_rdev & 0xFF;
    result.post_op_attr_u.attributes.fsid = buf.st_dev;

    /* Always truncate fsid to a 32-bit value, even though the fsid is
       defined as a uint64. We only use 32-bit variables for
       fsid/dev_t:s internally. This caused problems on systems where
       dev_t is signed, such as 32-bit OS X. */
    result.post_op_attr_u.attributes.fsid &= 0xFFFFFFFF;
    result.post_op_attr_u.attributes.fileid = buf.st_ino;
    result.post_op_attr_u.attributes.atime.seconds = buf.st_atime;
    result.post_op_attr_u.attributes.atime.nseconds = 0;
    result.post_op_attr_u.attributes.mtime.seconds = buf.st_mtime;
    result.post_op_attr_u.attributes.mtime.nseconds = 0;
    result.post_op_attr_u.attributes.ctime.seconds = buf.st_ctime;
    result.post_op_attr_u.attributes.ctime.nseconds = 0;

    return result;
}

/*
 * deduce mode from given settable attributes
 * default to rwxrwxr-x if no mode given
 */
mode_t create_mode(sattr3 new)
{
    if (new.mode.set_it == TRUE)
	return new.mode.set_mode3_u.mode;
    else
	return S_IRUSR | S_IWUSR | S_IXUSR | S_IRGRP | S_IWGRP | S_IXGRP |
	    S_IROTH | S_IXOTH;
}
