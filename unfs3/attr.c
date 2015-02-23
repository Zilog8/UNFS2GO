
/*
 * UNFS3 attribute handling
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */
 #include <utime.h> //utimbuf
 #include "error.c"

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
    if (go_lstat(path, &buf) < 0) {
		return error_attr;
    }
	
    return get_post_buf(buf, req);
}

/*
 * return pre-operation attributes
 */
pre_op_attr get_pre_buf(go_statstruct buf)
{
    pre_op_attr result;
	
    if (buf.st_dev==666) { //stat failed, so buf is a lie
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
 * return pre-operation attributes
 */
pre_op_attr get_pre(const char *path)
{
    pre_op_attr result;
	go_statstruct buf;
	
    if (go_lstat(path, &buf) < 0) {
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
    result.post_op_attr_u.attributes.rdev.specdata1 =
	(buf.st_rdev >> 8) & 0xFF;
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
 * return post-operation attributes, using fh for old dev/ino
 */
post_op_attr get_post_attr(const char *path, nfs_fh3 nfh,
			   struct svc_req * req)
{
    unfs3_fh_t *fh = (void *) nfh.data.data_val;

    return get_post(path, req);
}

/*
 * setting of time, races with local filesystem
 *
 * there is no futimes() function in POSIX or Linux
 */
static nfsstat3 set_time(const char *path, go_statstruct buf, sattr3 new)
{
    time_t new_atime, new_mtime;
    struct utimbuf utim;
    int res;

    /* set atime and mtime */
    if (new.atime.set_it != DONT_CHANGE || new.mtime.set_it != DONT_CHANGE) {

	/* compute atime to set */
	if (new.atime.set_it == SET_TO_SERVER_TIME)
	    new_atime = time(NULL);
	else if (new.atime.set_it == SET_TO_CLIENT_TIME)
	    new_atime = new.atime.set_atime_u.atime.seconds;
	else			       /* DONT_CHANGE */
	    new_atime = buf.st_atime;

	/* compute mtime to set */
	if (new.mtime.set_it == SET_TO_SERVER_TIME)
	    new_mtime = time(NULL);
	else if (new.mtime.set_it == SET_TO_CLIENT_TIME)
	    new_mtime = new.mtime.set_mtime_u.mtime.seconds;
	else			       /* DONT_CHANGE */
	    new_mtime = buf.st_mtime;

	utim.actime = new_atime;
	utim.modtime = new_mtime;

	res = go_utime(path, &utim);
	if (res == -1)
	    return setattr_err();
    }

    return NFS3_OK;
}

/*
 * race unsafe setting of attributes
 */
static nfsstat3 set_attr_unsafe(const char *path, nfs_fh3 nfh, sattr3 new)
{
    unfs3_fh_t *fh = (void *) nfh.data.data_val;
    uid_t new_uid;
    gid_t new_gid;
    go_statstruct buf;
    int res;

    res = go_lstat(path, &buf);
    if (res == -2)
	return NFS3ERR_NOENT;
    if (res == -1)
	return NFS3ERR_STALE;

    /* check local fs race */
    if (buf.st_ino != fh->ino)
	return NFS3ERR_STALE;

    /* set file size */
    if (new.size.set_it == TRUE) {
	res = go_truncate(path, new.size.set_size3_u.size);
	if (res == -1)
	    return setattr_err();
    }

    /* set uid and gid */
    if (new.uid.set_it == TRUE || new.gid.set_it == TRUE) {
	if (new.uid.set_it == TRUE)
	    new_uid = new.uid.set_uid3_u.uid;
	else
	    new_uid = -1;
	if (new_uid == buf.st_uid)
	    new_uid = -1;

	if (new.gid.set_it == TRUE)
	    new_gid = new.gid.set_gid3_u.gid;
	else
	    new_gid = -1;

	res = go_lchown(path, new_uid, new_gid);
	if (res == -1)
	    return setattr_err();
    }

    /* set mode */
    if (new.mode.set_it == TRUE) {
	res = go_chmod(path, new.mode.set_mode3_u.mode);
	if (res == -1)
	    return setattr_err();
    }

    return set_time(path, buf, new);
}

/*
 * set attributes of an object
 */
nfsstat3 set_attr(const char *path, nfs_fh3 nfh, sattr3 new)
{
    unfs3_fh_t *fh = (void *) nfh.data.data_val;
    int res, ores;
    uid_t new_uid;
    gid_t new_gid;
    go_statstruct buf;

    res = go_lstat(path, &buf);
    if (res == -2)
	return NFS3ERR_NOENT;
    if (res == -1)
	return NFS3ERR_STALE;

    /* 
     * don't open(2) device nodes, it could trigger
     * module loading on the server
     */
    if (S_ISBLK(buf.st_mode) || S_ISCHR(buf.st_mode))
	return set_attr_unsafe(path, nfh, new);

#ifdef S_ISLNK
    /* 
     * opening a symlink would open the underlying file,
     * don't try to do that
     */
    if (S_ISLNK(buf.st_mode))
	return set_attr_unsafe(path, nfh, new);
#endif

    /* 
     * open object for atomic setting of attributes
     */
    ores = go_open(path, UNFS3_FD_WRITE);
    if (ores == -1)
	ores = go_open(path, UNFS3_FD_READ);

    if (ores == -1)
	return set_attr_unsafe(path, nfh, new);

    res = go_lstat(path, &buf);
    if (res == -2) {
	return NFS3ERR_NOENT;
    }
    if (res == -1) {
	return NFS3ERR_STALE;
    }

    /* check local fs race */
    if (fh->ino != buf.st_ino) {
	return NFS3ERR_STALE;
    }

    /* set file size */
    if (new.size.set_it == TRUE) {
	res = go_truncate(path, new.size.set_size3_u.size);
	if (res == -1) {
	    return setattr_err();
	}
    }

    /* set uid and gid */
    if (new.uid.set_it == TRUE || new.gid.set_it == TRUE) {
	if (new.uid.set_it == TRUE)
	    new_uid = new.uid.set_uid3_u.uid;
	else
	    new_uid = -1;
	if (new_uid == buf.st_uid)
	    new_uid = -1;

	if (new.gid.set_it == TRUE)
	    new_gid = new.gid.set_gid3_u.gid;
	else
	    new_gid = -1;

	res = go_lchown(path, new_uid, new_gid);
	if (res == -1) {
	    return setattr_err();
	}
    }

    /* set mode */
    if (new.mode.set_it == TRUE) {
	res = go_chmod(path, new.mode.set_mode3_u.mode);
	if (res == -1) {
	    return setattr_err();
	}
    }

    /* finally, set times */
    return set_time(path, buf, new);
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
