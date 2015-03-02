/*
 * UNFS3 attribute handling
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */

#ifndef NFS_ATTR_H
#define NFS_ATTR_H
post_op_attr get_post_attr(const char *path, nfs_fh3 fh, struct svc_req *req);
post_op_attr get_post(const char *path, struct svc_req *req);
post_op_attr get_post_buf(go_statstruct buf, struct svc_req *req);
post_op_attr get_post_err();
pre_op_attr get_pre_buf(go_statstruct buf);
pre_op_attr  get_pre(const char *path);

mode_t create_mode(sattr3 sattr);
#endif
