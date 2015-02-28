#include <utime.h> //utimbuf
#include <errno.h>

int go_statvfs(const char *path, go_statvfsstruct * buf){
	errno = ENOSYS;
    return -1;
}

char *go_realpath(const char *path, char *resolved_path) {
	strcpy(resolved_path, path);
	return resolved_path;
}

int go_mksocket() {
	return go_nop("mksocket");
}

int go_readlink(const char *path, char *buf, size_t bufsiz) {
	return go_nop("readlink");	
}

int go_check_create_verifier(go_statstruct * buf, createverf3 verf){
    return go_nop("check");
}

int go_symlink(const char *oldpath, const char *newpath){
    return go_nop("symlink");
}

int go_mkfifo(const char *pathname, int mode){
    return go_nop("mkfifo");
}

go_mknod(const char *pathname, int mode, dev_t dev){
    return go_nop("mknod");
}

int go_link(const char *oldpath, const char *newpath){
    return go_nop("link");
}

