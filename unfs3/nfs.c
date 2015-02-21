
/*
 * UNFS3 NFS protocol procedures
 * (C) 2004, Pascal Schmidt
 * Copyright 2014 Karl Mikaelsson <derfian@cendio.se> for Cendio AB
 * see file LICENSE for license details
 */
 #include <utime.h> //utimbuf
 #include "readdir.c"
 
/*
 * resolve a filename into a path
 */
char *fh_decomp(nfs_fh3 fh)
{
    if (!nfh_valid(fh)) {
		return NULL;
    }
	
    unfs3_fh_t *obj = (void *) fh.data.data_val;
	
	if (obj->len == 0) //root
		return "/";

    return obj->path;
}

/*
 * cat an object name onto a path, checking for illegal input
 */
nfsstat3 cat_name(const char *path, const char *name, char *result)
{
    char *last;

    if (!path)
	return NFS3ERR_STALE;

    if (!name)
	return NFS3ERR_ACCES;

    if (name[0] == 0 || strchr(name, '/') != NULL)
	return NFS3ERR_ACCES;

    if (strlen(path) + strlen(name) + 2 > NFS_MAXPATHLEN)
	return NFS3ERR_NAMETOOLONG;

    if (strcmp(name, ".") == 0) {
	strcpy(result, path);
	return NFS3_OK;
    }

    sprintf(result, "%s/%s", path, name);
    return NFS3_OK;
}

void *nfsproc3_null_3_svc(U(void *argp), U(struct svc_req *rqstp))
{
    static void *result = NULL;

    return &result;
}

GETATTR3res *nfsproc3_getattr_3_svc(GETATTR3args * argp,
				    struct svc_req * rqstp)
{
    static GETATTR3res result;
    char *path;
    post_op_attr post;

    path = fh_decomp(argp->object);
    post = get_post(path, rqstp);

    result.status = NFS3_OK;
    result.GETATTR3res_u.resok.obj_attributes =
	post.post_op_attr_u.attributes;

    return &result;
}

/*
 * check ctime guard for SETATTR procedure
 */
static nfsstat3 in_sync(sattrguard3 guard, pre_op_attr pre)
{
    if (!pre.attributes_follow)
	return NFS3ERR_STALE;

    if (!guard.check)
	return NFS3_OK;

    if (guard.sattrguard3_u.obj_ctime.seconds !=
	pre.pre_op_attr_u.attributes.ctime.seconds)
	return NFS3ERR_NOT_SYNC;

    return NFS3_OK;
}

SETATTR3res *nfsproc3_setattr_3_svc(SETATTR3args * argp,
				    struct svc_req * rqstp)
{
    static SETATTR3res result;
    pre_op_attr pre;
    char *path;
	
	go_statstruct stbuf;
    path = fh_decomp(argp->object);
	go_lstat(path, &stbuf);
    pre = get_pre(path);
    result.status = join(in_sync(argp->guard, pre), exports_rw());

    if (result.status == NFS3_OK)
	result.status = set_attr(path, argp->object, argp->new_attributes);

    /* overlaps with resfail */
    result.SETATTR3res_u.resok.obj_wcc.before = pre;
    result.SETATTR3res_u.resok.obj_wcc.after =
		get_post_ll(path, rqstp);

    return &result;
}

LOOKUP3res *nfsproc3_lookup_3_svc(LOOKUP3args * argp, struct svc_req * rqstp)
{
    static LOOKUP3res result;
    unfs3_fh_t *fh;
    char *path;
    char obj[NFS_MAXPATHLEN];
    go_statstruct buf;
    int res;

	path = fh_decomp(argp->what.dir);

    result.status = cat_name(path, argp->what.name, obj);
    if (result.status == NFS3_OK) {
		res = go_lstat(obj, &buf);
			if (res < 0){
				if (res == -2) { 
					errno = ENOENT;
				}
				result.status = lookup_err();
			}	else {
				fh = fh_extend(argp->what.dir, buf.st_ino, obj);

				if (fh) {
					result.LOOKUP3res_u.resok.object.data.data_len = fh_length(fh);
					result.LOOKUP3res_u.resok.object.data.data_val = (char *) fh;
					result.LOOKUP3res_u.resok.obj_attributes = get_post_buf(buf, rqstp);
				} else {
				result.status = NFS3ERR_NAMETOOLONG;
				}
			}
    }
	
    /* overlaps with resfail */
    result.LOOKUP3res_u.resok.dir_attributes = get_post_ll(path, rqstp);
    return &result;
}

ACCESS3res *nfsproc3_access_3_svc(ACCESS3args * argp, struct svc_req * rqstp)
{
    static ACCESS3res result;
    char *path;
    post_op_attr post;
    int newaccess = 0;

    path = fh_decomp(argp->object);
    post = get_post(path, rqstp);

    /* allow everything */
    newaccess |= ACCESS3_READ | ACCESS3_MODIFY | ACCESS3_EXTEND | ACCESS3_EXECUTE;

    /* adjust if directory */
    if (post.post_op_attr_u.attributes.type == NF3DIR) {
	if (newaccess & (ACCESS3_READ | ACCESS3_EXECUTE))
	    newaccess |= ACCESS3_LOOKUP;
	if (newaccess & ACCESS3_MODIFY)
	    newaccess |= ACCESS3_DELETE;
	newaccess &= ~ACCESS3_EXECUTE;
    }

    result.status = NFS3_OK;
    result.ACCESS3res_u.resok.access = newaccess;
    result.ACCESS3res_u.resok.obj_attributes = post;

    return &result;
}

READLINK3res *nfsproc3_readlink_3_svc(READLINK3args * argp,
				      struct svc_req * rqstp)
{
    static READLINK3res result;
    char *path;
    static char buf[NFS_MAXPATHLEN];
    int res;

	go_statstruct stbuf;
    path = fh_decomp(argp->symlink);
	go_lstat(path, &stbuf);

    res = go_readlink(path, buf, NFS_MAXPATHLEN - 1);
    if (res == -1)
	result.status = readlink_err();
    else {
	/* readlink does not NULL-terminate */
	buf[res] = 0;

	result.status = NFS3_OK;
	result.READLINK3res_u.resok.data = buf;
    }

    /* overlaps with resfail */
    result.READLINK3res_u.resok.symlink_attributes =
		get_post_ll(path, rqstp);

    return &result;
}

READ3res *nfsproc3_read_3_svc(READ3args * argp, struct svc_req * rqstp)
{
    static READ3res result;
    char *path;
    int ores, res;
    static char buf[NFS_MAXDATA_TCP + 1];
    unsigned int maxdata;

    if (get_socket_type(rqstp) == SOCK_STREAM)
	maxdata = NFS_MAXDATA_TCP;
    else
	maxdata = NFS_MAXDATA_UDP;

    go_statstruct stbuf;
	path = fh_decomp(argp->file);
	go_lstat(path, &stbuf);
    result.status = is_reg(path);

    /* if bigger than rtmax, truncate length */
    if (argp->count > maxdata)
	argp->count = maxdata;

    if (result.status == NFS3_OK) {
	ores = go_open(path, UNFS3_FD_READ);
	if (ores > -1) {
	    /* read one more to check for eof */
	    res = go_pread(path, buf, argp->count + 1, argp->offset);

	    /* eof if we could not read one more */
	    result.READ3res_u.resok.eof = (res <= (int64) argp->count);

	    /* readjust count if not eof */
	    if (!result.READ3res_u.resok.eof) {
			res--;
	    }

	    if (res > -1) {
		result.READ3res_u.resok.count = res;
		result.READ3res_u.resok.data.data_len = res;
		result.READ3res_u.resok.data.data_val = buf;
	    } else {
		/* error during read() */

		/* EINVAL means unreadable object */
		if (errno == EINVAL)
		    result.status = NFS3ERR_INVAL;
		else
		    result.status = NFS3ERR_IO;
	    }
	} else
	    /* opening for read failed */
	    result.status = read_err();
    }

    /* overlaps with resfail */
    result.READ3res_u.resok.file_attributes =
		get_post_ll(path, rqstp);
    return &result;
}

WRITE3res *nfsproc3_write_3_svc(WRITE3args * argp, struct svc_req * rqstp)
{
    static WRITE3res result;
    char *path;
    int ores, res;

    go_statstruct stbuf;
	path = fh_decomp(argp->file);
	go_lstat(path, &stbuf);
	pre_op_attr pre;
	pre = get_pre(path);
    result.status = join(is_reg(path), exports_rw());

    if (result.status == NFS3_OK) {
		ores = go_open(path, UNFS3_FD_WRITE);
		if (ores != -1) {
			res =
			go_pwrite(path, argp->data.data_val, argp->data.data_len,
					   argp->offset);

			/* we always do fsync(), never fdatasync() */
			if (argp->stable == DATA_SYNC)
			argp->stable = FILE_SYNC;

			if (res != -1) {
			result.WRITE3res_u.resok.count = res;
			result.WRITE3res_u.resok.committed = argp->stable;
			memcpy(result.WRITE3res_u.resok.verf, wverf,
				   NFS3_WRITEVERFSIZE);
			} else {
			/* error during write or close */
			result.status = write_write_err();
			}
		} else
			/* could not open for writing */
			result.status = write_open_err();
    }

    /* overlaps with resfail */
    result.WRITE3res_u.resok.file_wcc.before = pre;
    result.WRITE3res_u.resok.file_wcc.after = 
		get_post_ll(path, rqstp);
    return &result;
}

/*
 * store verifier in atime and mtime 
 */
static int store_create_verifier(char *obj, createverf3 verf)
{
    struct utimbuf ubuf;

    ubuf.actime = verf[0] | verf[1] << 8 | verf[2] << 16 | verf[3] << 24;
    ubuf.modtime = verf[4] | verf[5] << 8 | verf[6] << 16 | verf[7] << 24;

    return go_utime(obj, &ubuf);
}

/*
 * check if a create verifier matches
 */
static int check_create_verifier(go_statstruct * buf, createverf3 verf)
{
    return ((buf->st_atime ==
	     (verf[0] | verf[1] << 8 | verf[2] << 16 | verf[3] << 24))
	    && (buf->st_mtime ==
		(verf[4] | verf[5] << 8 | verf[6] << 16 | verf[7] << 24)));
}

CREATE3res *nfsproc3_create_3_svc(CREATE3args * argp, struct svc_req * rqstp)
{
    static CREATE3res result;
    char *path;
    char obj[NFS_MAXPATHLEN];
    sattr3 new_attr;
    int ores = -1, res = -1;
    go_statstruct buf;
    uint32 gen;
    int flags = O_RDWR | O_CREAT | O_TRUNC | O_NONBLOCK;

	go_statstruct stbuf;
    path = fh_decomp(argp->where.dir);
	go_lstat(path, &stbuf);

	pre_op_attr pre;
	pre = get_pre(path);
    
    result.status = join(cat_name(path, argp->where.name, obj), exports_rw());

    /* GUARDED and EXCLUSIVE maps to Unix exclusive create */
    if (argp->how.mode != UNCHECKED)
	flags = flags | O_EXCL;

    if (argp->how.mode != EXCLUSIVE) {
	new_attr = argp->how.createhow3_u.obj_attributes;
    }

    /* Try to open the file */
    if (result.status == NFS3_OK) {
	    ores = go_open_create(obj, flags, create_mode(new_attr));
	}

    if (ores > 0) {
		//fprintf(stderr,  "NFS3 Create: Successful open\n");
	res = go_lstat(obj, &buf);
		if (res > -1) {
		//fprintf(stderr,  "NFS3 Create: Successful stat\n");

	    result.CREATE3res_u.resok.obj =
		fh_extend_post(argp->where.dir, buf.st_ino, obj);
	    result.CREATE3res_u.resok.obj_attributes =
		get_post_buf(buf, rqstp);
		} else if (res==-2) {
			result.status = NFS3ERR_NOENT;
			fprintf(stderr,  "NFS3 Create: stat() failed\n");
	} else {
			fprintf(stderr,  "NFS3 Create: Stat result i%\n", res);
			fprintf(stderr,  "NFS3 Create: go_lstat() or go_store_create_verifier() failed\n");
			result.status = NFS3ERR_IO;
	}
    } else if (result.status == NFS3_OK) {
	
	//fprintf(stderr,  "open() failed\n");
	if (argp->how.mode == EXCLUSIVE && errno == EEXIST) {
	    
	//fprintf(stderr,  "Check if verifier matches\n");
	    ores = go_open(obj, UNFS3_FD_WRITE);
	    if (ores > -1) {
			res = go_lstat(obj, &buf);
	    }

	    if (res > -1) {
		if (go_check_create_verifier
		    (&buf, argp->how.createhow3_u.verf)) {
	//fprintf(stderr,  "The verifier matched. Return success\n");
		    result.CREATE3res_u.resok.obj =
			fh_extend_post(argp->where.dir, buf.st_ino, obj);
		    result.CREATE3res_u.resok.obj_attributes =
			get_post_buf(buf, rqstp);
		} else {
	//fprintf(stderr,  "The verifier doesn't match\n");
		    result.status = NFS3ERR_EXIST;
		}
	    }
	}
	if (res < 0) {
		if (res == -2) { errno = ENOENT;}
	    result.status = create_err();
	}
    }

	//fprintf(stderr,  "overlaps with resfail\n");
    result.CREATE3res_u.resok.dir_wcc.before = pre;
    result.CREATE3res_u.resok.dir_wcc.after = 
		get_post_ll(path, rqstp);

    return &result;
}

MKDIR3res *nfsproc3_mkdir_3_svc(MKDIR3args * argp, struct svc_req * rqstp)
{
    static MKDIR3res result;
    char *path;
    pre_op_attr pre;
    post_op_attr post;
    char obj[NFS_MAXPATHLEN];
    int res;

    path = fh_decomp(argp->where.dir);
    pre = get_pre(path);
    result.status =
	join(cat_name(path, argp->where.name, obj), exports_rw());

    if (result.status == NFS3_OK) {
	res = go_mkdir(obj, create_mode(argp->attributes));
	if (res == -1)
	    result.status = mkdir_err();
	else {
	    result.MKDIR3res_u.resok.obj =
		fh_extend_type(argp->where.dir, obj, S_IFDIR);
	    result.MKDIR3res_u.resok.obj_attributes = get_post(obj, rqstp);
	}
    }

    post = get_post_attr(path, argp->where.dir, rqstp);

    /* overlaps with resfail */
    result.MKDIR3res_u.resok.dir_wcc.before = pre;
    result.MKDIR3res_u.resok.dir_wcc.after = post;

    return &result;
}

SYMLINK3res *nfsproc3_symlink_3_svc(SYMLINK3args * argp,
				    struct svc_req * rqstp)
{
    static SYMLINK3res result;
    char *path;
    pre_op_attr pre;
    post_op_attr post;
    char obj[NFS_MAXPATHLEN];
    int res;
    mode_t new_mode;

    path = fh_decomp(argp->where.dir);
    pre = get_pre(path);
    result.status =
	join(cat_name(path, argp->where.name, obj), exports_rw());

    if (argp->symlink.symlink_attributes.mode.set_it == TRUE)
	new_mode = create_mode(argp->symlink.symlink_attributes);
    else {
	/* default rwxrwxrwx */
	new_mode =
	    S_IRUSR | S_IWUSR | S_IXUSR | S_IRGRP | S_IWGRP | S_IXGRP |
	    S_IROTH | S_IWOTH | S_IXOTH;
    }

    if (result.status == NFS3_OK) {
	umask(~new_mode);
	res = go_symlink(argp->symlink.symlink_data, obj);
	umask(0);
	if (res == -1)
	    result.status = symlink_err();
	else {
	    result.SYMLINK3res_u.resok.obj =
		fh_extend_type(argp->where.dir, obj, S_IFLNK);
	    result.SYMLINK3res_u.resok.obj_attributes =
		get_post(obj, rqstp);
	}
    }

    post = get_post_attr(path, argp->where.dir, rqstp);

    /* overlaps with resfail */
    result.SYMLINK3res_u.resok.dir_wcc.before = pre;
    result.SYMLINK3res_u.resok.dir_wcc.after = post;

    return &result;
}

/*
 * create Unix socket
 */
static int mksocket(const char *path, mode_t mode)
{
    int res, sock;
    struct sockaddr_un addr;

    sock = socket(PF_UNIX, SOCK_STREAM, 0);
    addr.sun_family = AF_UNIX;
    strcpy(addr.sun_path, path);
    res = sock;
    if (res != -1) {
	umask(~mode);
	res =
	    bind(sock, (struct sockaddr *) &addr,
		 sizeof(addr.sun_family) + strlen(addr.sun_path));
	umask(0);
	close(sock);
    }
    return res;
}

/*
 * check and process arguments to MKNOD procedure
 */
static nfsstat3 mknod_args(mknoddata3 what, const char *obj, mode_t * mode,
			   dev_t * dev)
{
    sattr3 attr;

    /* determine attributes */
    switch (what.type) {
	case NF3REG:
	case NF3DIR:
	case NF3LNK:
	    return NFS3ERR_INVAL;
	case NF3SOCK:
	    if (strlen(obj) + 1 > UNIX_PATH_MAX)
		return NFS3ERR_NAMETOOLONG;
	    /* fall thru */
	case NF3FIFO:
	    attr = what.mknoddata3_u.pipe_attributes;
	    break;
	case NF3BLK:
	case NF3CHR:
	    attr = what.mknoddata3_u.device.dev_attributes;
	    *dev = (what.mknoddata3_u.device.spec.specdata1 << 8)
		+ what.mknoddata3_u.device.spec.specdata2;
	    break;
    }

    *mode = create_mode(attr);

    /* adjust mode for creation of device special files */
    switch (what.type) {
	case NF3CHR:
	    *mode |= S_IFCHR;
	    break;
	case NF3BLK:
	    *mode |= S_IFBLK;
	    break;
	default:
	    break;
    }

    return NFS3_OK;
}

MKNOD3res *nfsproc3_mknod_3_svc(MKNOD3args * argp, struct svc_req * rqstp)
{
    static MKNOD3res result;
    char *path;
    pre_op_attr pre;
    post_op_attr post;
    char obj[NFS_MAXPATHLEN];
    int res;
    mode_t new_mode = 0;
    dev_t dev = 0;

    path = fh_decomp(argp->where.dir);
    pre = get_pre(path);
    result.status =
	join3(cat_name(path, argp->where.name, obj),
	      mknod_args(argp->what, obj, &new_mode, &dev), exports_rw());

    if (result.status == NFS3_OK) {
	if (argp->what.type == NF3CHR || argp->what.type == NF3BLK)
	    res = go_mknod(obj, new_mode, dev);	/* device */
	else if (argp->what.type == NF3FIFO)
	    res = go_mkfifo(obj, new_mode);	/* FIFO */
	else
	    res = go_mksocket(obj, new_mode);	/* socket */

	if (res == -1) {
	    result.status = mknod_err();
	} else {
	    result.MKNOD3res_u.resok.obj =
		fh_extend_type(argp->where.dir, obj, type_to_mode(argp->what.type));
	    result.MKNOD3res_u.resok.obj_attributes = get_post(obj, rqstp);
	}
    }

    post = get_post_attr(path, argp->where.dir, rqstp);

    /* overlaps with resfail */
    result.MKNOD3res_u.resok.dir_wcc.before = pre;
    result.MKNOD3res_u.resok.dir_wcc.after = post;

    return &result;
}

REMOVE3res *nfsproc3_remove_3_svc(REMOVE3args * argp, struct svc_req * rqstp)
{
    static REMOVE3res result;
    char *path;
    char obj[NFS_MAXPATHLEN];
    int res;

	go_statstruct stbuf;
    path = fh_decomp(argp->object.dir);
	go_lstat(path, &stbuf);
	pre_op_attr pre;
	pre = get_pre(path);
    
    result.status =
	join(cat_name(path, argp->object.name, obj), exports_rw());

    if (result.status == NFS3_OK) {
        change_readdir_cookie();
	res = go_remove(obj);
	if (res == -1)
	    result.status = remove_err();
    }

    /* overlaps with resfail */
    result.REMOVE3res_u.resok.dir_wcc.before = pre;
    result.REMOVE3res_u.resok.dir_wcc.after = 
		get_post_ll(path, rqstp);
    return &result;
}

RMDIR3res *nfsproc3_rmdir_3_svc(RMDIR3args * argp, struct svc_req * rqstp)
{
    static RMDIR3res result;
    char *path;
    char obj[NFS_MAXPATHLEN];
    int res;

	go_statstruct stbuf;
    path = fh_decomp(argp->object.dir);
	go_lstat(path, &stbuf);
	pre_op_attr pre;
	pre = get_pre(path);
    
    result.status =
	join(cat_name(path, argp->object.name, obj), exports_rw());

    if (result.status == NFS3_OK) {
        change_readdir_cookie();
	res = go_rmdir(obj);
	if (res < 0)
	    result.status = rmdir_err();
    }

    /* overlaps with resfail */
    result.RMDIR3res_u.resok.dir_wcc.before = pre;
    result.RMDIR3res_u.resok.dir_wcc.after = 
		get_post_ll(path, rqstp);
    return &result;
}

RENAME3res *nfsproc3_rename_3_svc(RENAME3args * argp, struct svc_req * rqstp)
{
    static RENAME3res result;
    char *from;
    char *to;
    char from_obj[NFS_MAXPATHLEN];
    char to_obj[NFS_MAXPATHLEN];
    post_op_attr post;
    int res;

    from = fh_decomp(argp->from.dir);
	
    pre_op_attr from_pre;
    from_pre = get_pre(from);
	
    result.status = join(cat_name(from, argp->from.name, from_obj), exports_rw());

	go_statstruct stbuf;
    to = fh_decomp(argp->to.dir);
	go_lstat(to, &stbuf);
	
	pre_op_attr to_pre;
	to_pre = get_pre(to);
    

    if (result.status == NFS3_OK) {
	result.status =
	    join(cat_name(to, argp->to.name, to_obj), NFS3_OK);

	if (result.status == NFS3_OK) {
	    change_readdir_cookie();
	    res = go_rename(from_obj, to_obj);
		if (res == -2)
			errno = ENOENT;
	    if (res == -1)
		result.status = rename_err();
	}
    }

    post = get_post_attr(from, argp->from.dir, rqstp);

    /* overlaps with resfail */
    result.RENAME3res_u.resok.fromdir_wcc.before = from_pre;
    result.RENAME3res_u.resok.fromdir_wcc.after = post;
    result.RENAME3res_u.resok.todir_wcc.before = to_pre;
    result.RENAME3res_u.resok.todir_wcc.after = 
		get_post_ll(to, rqstp);

    return &result;
}

LINK3res *nfsproc3_link_3_svc(LINK3args * argp, struct svc_req * rqstp)
{
    static LINK3res result;
    char *path, *old;
    pre_op_attr pre;
    post_op_attr post;
    char obj[NFS_MAXPATHLEN];
    int res;

    path = fh_decomp(argp->link.dir);
    pre = get_pre(path);
    result.status = join(cat_name(path, argp->link.name, obj), exports_rw());

    go_statstruct stbuf;
	old = fh_decomp(argp->file);
	go_lstat(old, &stbuf);

    if (old && result.status == NFS3_OK) {
	result.status = NFS3_OK;

	if (result.status == NFS3_OK) {
	    res = go_link(old, obj);
	    if (res == -1)
		result.status = link_err();
	}
    } else if (!old)
	result.status = NFS3ERR_STALE;

    post = get_post_attr(path, argp->link.dir, rqstp);

    /* overlaps with resfail */
    result.LINK3res_u.resok.file_attributes =
		get_post_ll(old, rqstp);
    result.LINK3res_u.resok.linkdir_wcc.before = pre;
    result.LINK3res_u.resok.linkdir_wcc.after = post;

    return &result;
}

READDIR3res *nfsproc3_readdir_3_svc(READDIR3args * argp,
				    struct svc_req * rqstp)
{
    static READDIR3res result;
    char *path;

	go_statstruct stbuf;
    path = fh_decomp(argp->dir);
	go_lstat(path, &stbuf);
    result = read_dir(path, argp->cookie, argp->cookieverf, argp->count);
    result.READDIR3res_u.resok.dir_attributes = 
		get_post_ll(path, rqstp);

    return &result;
}

READDIRPLUS3res *nfsproc3_readdirplus_3_svc(U(READDIRPLUS3args * argp),
					    U(struct svc_req * rqstp))
{
    static READDIRPLUS3res result;

    /* 
     * we don't do READDIRPLUS since it involves filehandle and
     * attribute getting which is impossible to do atomically
     * from user-space
     */
    result.status = NFS3ERR_NOTSUPP;
    result.READDIRPLUS3res_u.resfail.dir_attributes.attributes_follow = FALSE;

    return &result;
}

FSSTAT3res *nfsproc3_fsstat_3_svc(FSSTAT3args * argp, struct svc_req * rqstp)
{
    static FSSTAT3res result;
    char *path;
    go_statvfsstruct buf;
    int res;

    path = fh_decomp(argp->fsroot);

    /* overlaps with resfail */
    result.FSSTAT3res_u.resok.obj_attributes = get_post(path, rqstp);

    res = go_statvfs(path, &buf);
    if (res == -1) {
	/* statvfs fell on its nose */
	if ((exports_opts & OPT_REMOVABLE) && export_point(path)) {
	    /* Removable media export point; probably no media inserted.
	       Return dummy values. */
	    result.status = NFS3_OK;
	    result.FSSTAT3res_u.resok.tbytes = 0;
	    result.FSSTAT3res_u.resok.fbytes = 0;
	    result.FSSTAT3res_u.resok.abytes = 0;
	    result.FSSTAT3res_u.resok.tfiles = 0;
	    result.FSSTAT3res_u.resok.ffiles = 0;
	    result.FSSTAT3res_u.resok.afiles = 0;
	    result.FSSTAT3res_u.resok.invarsec = 0;
	} else {
	    result.status = NFS3ERR_IO;
	}
    } else {
	result.status = NFS3_OK;
	result.FSSTAT3res_u.resok.tbytes =
	    (uint64) buf.f_blocks * buf.f_frsize;
	result.FSSTAT3res_u.resok.fbytes = 
	    (uint64) buf.f_bfree * buf.f_frsize;
	result.FSSTAT3res_u.resok.abytes =
	    (uint64) buf.f_bavail * buf.f_frsize;
	result.FSSTAT3res_u.resok.tfiles = buf.f_files;
	result.FSSTAT3res_u.resok.ffiles = buf.f_ffree;
	result.FSSTAT3res_u.resok.afiles = buf.f_ffree;
	result.FSSTAT3res_u.resok.invarsec = 0;
    }

    return &result;
}

FSINFO3res *nfsproc3_fsinfo_3_svc(FSINFO3args * argp, struct svc_req * rqstp)
{
    static FSINFO3res result;
    char *path;
    unsigned int maxdata;

    if (get_socket_type(rqstp) == SOCK_STREAM)
	maxdata = NFS_MAXDATA_TCP;
    else
	maxdata = NFS_MAXDATA_UDP;

    path = fh_decomp(argp->fsroot);

    result.FSINFO3res_u.resok.obj_attributes = get_post(path, rqstp);

    result.status = NFS3_OK;
    result.FSINFO3res_u.resok.rtmax = maxdata;
    result.FSINFO3res_u.resok.rtpref = maxdata;
    result.FSINFO3res_u.resok.rtmult = 4096;
    result.FSINFO3res_u.resok.wtmax = maxdata;
    result.FSINFO3res_u.resok.wtpref = maxdata;
    result.FSINFO3res_u.resok.wtmult = 4096;
    result.FSINFO3res_u.resok.dtpref = 4096;
    result.FSINFO3res_u.resok.maxfilesize = ~0ULL;
    result.FSINFO3res_u.resok.time_delta.seconds = go_time_delta_seconds;
    result.FSINFO3res_u.resok.time_delta.nseconds = 0;
    result.FSINFO3res_u.resok.properties = go_fsinfo_properties;

    return &result;
}

PATHCONF3res *nfsproc3_pathconf_3_svc(PATHCONF3args * argp,
				      struct svc_req * rqstp)
{
    static PATHCONF3res result;
    char *path;

    path = fh_decomp(argp->object);

    result.PATHCONF3res_u.resok.obj_attributes = get_post(path, rqstp);

    result.status = NFS3_OK;
    result.PATHCONF3res_u.resok.linkmax = 0xFFFFFFFF;
    result.PATHCONF3res_u.resok.name_max = NFS_MAXPATHLEN;
    result.PATHCONF3res_u.resok.no_trunc = TRUE;
    result.PATHCONF3res_u.resok.chown_restricted = FALSE;
    result.PATHCONF3res_u.resok.case_insensitive =
	go_pathconf_case_insensitive;
    result.PATHCONF3res_u.resok.case_preserving = TRUE;

    return &result;
}

COMMIT3res *nfsproc3_commit_3_svc(COMMIT3args * argp, struct svc_req * rqstp)
{
    static COMMIT3res result;
    char *path;

    go_statstruct stbuf;
    path = fh_decomp(argp->file);
	go_lstat(path, &stbuf);
	pre_op_attr pre;
	pre = get_pre(path);
    result.status = join(is_reg(path), exports_rw());

    if (result.status == NFS3_OK) {
		memcpy(result.COMMIT3res_u.resok.verf, wverf, NFS3_WRITEVERFSIZE);
	}
    /* overlaps with resfail */
    result.COMMIT3res_u.resfail.file_wcc.before = pre;
    result.COMMIT3res_u.resfail.file_wcc.after = 
		get_post_ll(path, rqstp);

    return &result;
}
