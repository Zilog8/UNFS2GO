/*
 * UNFS3 low-level filehandle routines
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */

#ifndef UNFS3_FH_H
#define UNFS3_FH_H

/* minimum length of complete filehandle */
#define FH_MINLEN 9

#ifdef __GNUC__
typedef struct {
	uint64			ino;
	unsigned char	len;
} __attribute__((packed)) unfs3_fh_t;
#else
#pragma pack(1)
typedef struct {
	uint64			ino;
	unsigned char	len;
} unfs3_fh_t;
#pragma pack(4)
#endif

int nfh_valid(nfs_fh3 fh);

u_int fh_length(const unfs3_fh_t *fh);

char *fh_decomp(nfs_fh3 fh);
unfs3_fh_t *fh_comp(uint64 ino, const char *path);
post_op_fh3 fh_comp_post(uint64 ino, const char *path);
post_op_fh3 fh_comp_type(const char *path, unsigned int type);
#endif
