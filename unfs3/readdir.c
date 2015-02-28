
/*
 * UNFS3 readdir routine
 * (C) 2004, Pascal Schmidt
 * see file LICENSE for license details
 */

#include "error.c"

/*
 * maximum number of entries in readdir results
 *
 * this is 4096 / 24 (the minimum size of an entry3)
 */
#define MAX_ENTRIES 170

/*
 * static READDIR3resok size with XDR overhead
 *
 * 88 bytes attributes, 8 bytes verifier, 4 bytes value_follows for
 * first entry, 4 bytes eof flag
 */
#define RESOK_SIZE 104

/*
 * static entry3 size with XDR overhead
 *
 * 8 bytes fileid, 4 bytes name length, 8 bytes cookie, 4 byte value_follows
 */
#define ENTRY_SIZE 24

/*
 * size of a name with XDR overhead
 *
 * XDR pads to multiple of 4 bytes
 */
#define NAME_SIZE(x) (((strlen((x))+3)/4)*4)

uint32 directory_hash(const char *path)
{
    go_dirstream *search;
    struct dirent *this;
    uint32 hval = 0;

    search = go_opendir(path);
    if (!search) {
	return 0;
    }

    while ((this = go_readdir(search)) != NULL) {
	hval = fnv1a_32(this->d_name, hval);
    }

    go_closedir(search);
    return hval;
}

/*
 * perform a READDIR operation
 */
READDIR3res read_dir(const char *path, cookie3 cookie, cookieverf3 verf, count3 count)
{
    READDIR3res result;
    READDIR3resok resok;
    static entry3 entry[MAX_ENTRIES];
    go_statstruct buf;
    go_dirstream *search;
    struct dirent *this;
    count3 i, real_count;
    static char obj[NFS_MAXPATHLEN * MAX_ENTRIES];
    char scratch[NFS_MAXPATHLEN];

    /* we refuse to return more than 4k from READDIR */
    if (count > 4096)
	count = 4096;

    /* account for size of information heading resok structure */
    real_count = RESOK_SIZE;

	//TODO: Restrain returned bytes by 'count' and 'real_count'
	//TODO: Give a clear signal of eof vs. not eof
	go_readdir_full(path, obj, entry, NFS_MAXPATHLEN, MAX_ENTRIES);

    if (entry[0].name)
	resok.reply.entries = &entry[0];
    else
	resok.reply.entries = NULL;

    if (this)
	resok.reply.eof = FALSE;
    else
	resok.reply.eof = TRUE;

    memset(verf, 0, NFS3_COOKIEVERFSIZE);
    memcpy(resok.cookieverf, verf, NFS3_COOKIEVERFSIZE);

    result.status = NFS3_OK;
    result.READDIR3res_u.resok = resok;

    return result;
}
