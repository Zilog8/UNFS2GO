#include <utime.h> //utimbuf
#include <errno.h>

char *go_readdir_helper(char *, int);

go_DIR *go_opendir(const char *name) {
	int count = go_opendir_helper(name);
	if(count!=-1) {
		go_DIR *ret;
		ret = malloc(sizeof(go_DIR));
		ret->entryIndex = 0;
		ret->count = count;
		ret->directory_path = malloc(strlen(name)+1);
		strcpy(ret->directory_path, name);
		//fprintf(stderr, "opendir on %s\n",  ret->directory_path);
		return ret;
	}
	fprintf(stderr, "opendir failed on %s\n",  name);
	return NULL;
}

dirent *go_readdir(go_DIR * dir) {
		//fprintf(stderr, "readdir 1 %s\n",  dir->directory_path);
	
	if (dir->entryIndex >= dir->count) {
		//fprintf(stderr, "readdir too far on %s\n", dir->directory_path);
		return NULL;
	}
		//fprintf(stderr, "readdir 2 %s\n",  dir->directory_path);
	
	char *dn = go_readdir_helper(dir->directory_path,dir->entryIndex);
	if(strcmp(dn,"")!=0) {
		//fprintf(stderr, "readdir probably succeded %s to %s\n", dir->directory_path, dn);
		dirent *ret;
		ret = malloc(sizeof(dirent));
		ret->d_name = dn;
		dir->entryIndex = 1 + dir->entryIndex;
		return ret;
	}
	fprintf(stderr, "readdir probably failed %s to %s\n", dir->directory_path, dn);
	return NULL;
}

int go_closedir(go_DIR * dir) {
	free(dir->directory_path); //I think this it how it works
    free(dir);
	return 0;
}

char *go_locate_file(uint32 dev, uint64 ino) {
	char *path;
	return path;
}

int go_utime(const char *path, const struct utimbuf *times) {
    return go_utime_helper(path, times->actime, times->modtime);
}

int go_rmdir(const char *path){
	int retV = go_rmdir_helper(path);
    if (retV == -2) {
		errno = ENOTEMPTY;
		return -1;
	}
	return retV;
}

//Functions exported from Go
/*
int go_init() {
    return go_nop();
}

int go_open(const char *pathname, int flags) {
    return go_nop();
}

int go_close(int fd) {
	return go_nop();
}

int go_lstat(const char *file_name, go_statstruct * buf)
{
	return go_nop();

	
int go_fstat(int fd, go_statstruct * buf)
{
	return go_nop();
}

int go_pread(int fd, char *buf, count3 count, offset3 offset) {
 return go_nop();
}

int go_mkdir(const char *pathname, int mode){
    return go_nop("mkdir");
}

int go_rmdir(const char *path){
    return go_nop("rmdir");
}


int go_remove(char *pathname) {
	return go_nop("remove");
}

int go_open_create(const char *pathname, int flags, int mode) {
    return go_nop("open_create");
}

ssize_t go_pwrite(int fd, const void *buf, size_t count, offset3 offset) {
	return go_nop("pwrite");
}

int go_fsync(int fd)
{
	return go_nop("fsync");
}

int go_ftruncate(int fd, offset3 length) {
	return go_nop("ftruncate");
}

int go_truncate(const char *path, offset3 length) {
	return go_nop("truncate");
}

int go_rename(const char *oldpath, const char *newpath){
    return go_nop("rename");
}

int go_fchmod(int fd, int mode) {
	return go_nop("fchmod");
}

*/

int go_statvfs(const char *path, backend_statvfsstruct * buf){
	errno = ENOSYS;
    return -1;
}

void go_shutdown() {
	return;
}

char *go_realpath(const char *path, char *resolved_path) {
	strcpy(resolved_path, path);
	return resolved_path;
}

int go_mksocket() {
	return go_nop("mksocket");
}

int go_chmod(const char *path, int mode) {
	return go_nop("chmod");
}

int go_lchmod(const char *path, int mode) {
	return go_nop("lchmod");
}

int go_fchown(int fd, uid_t owner, gid_t group) {
	return go_nop("fchown");
}

int go_chown(const char *path, uid_t owner, gid_t group) {
	return go_nop("chown");
}

int go_lchown(const char *path, uid_t owner, gid_t group) {
	return go_nop("lchown");
}

int go_readlink(const char *path, char *buf, size_t bufsiz) {
	return go_nop("readlink");	
}

int go_store_create_verifier(char *obj, createverf3 verf){
    return go_nop("store_create_verifier");
}

int go_check_create_verifier(backend_statstruct * buf, createverf3 verf){
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

