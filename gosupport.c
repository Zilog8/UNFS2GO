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

int go_rmdir(const char *path){
	int retV = go_rmdir_helper(path);
    if (retV == -2) {
		errno = ENOTEMPTY;
		return -1;
	}
	return retV;
}

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

