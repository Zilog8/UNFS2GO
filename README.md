UNFS2GO

My first "project" in CGO (and C for that matter);
A fork of UNFS3 (Usermode NFS3 server) that uses Go backends.

The Go backends are abstracted through the minfs.MinFS interface.

Why:

I have a server without FUSE and whose kernel I can't modify.
It can mount NFS shares however, so I started wondering if NFS could be
used as a poor-man's FUSE?  Upon seeing the complexity of the NFS protocol,
I figured my best bet was to re-purposed a pre-existing NFS server.
Since I couldn't find an NFS server in Go, I settled on UNFS3, a usermode 
server written in C. Because of the simplicity of CGO, it actually wasn't
too difficult to bring this gruesome chimera into existence (despite my
utter lack of C knowledge).

Build:

	go build unfs2go.go unfs2go_exports.go

Dependencies:

You'll probably need the rcpbind and nfs-common packages.

In debian:

	sudo apt-get install rpcbind nfs-common
	
Usage:
The first argument gives the bind type, with additional arguments
determined by the individual bind type. 

	unfs2go -o ./sharedDir

If you want to use the shimFS (acts as a cache-ing layer for another
backend. Might be useful for network filesystems) it has to be the first
argument:

	unfs2go -s -z ./zipfile.zip
	
Backends:

bind type  | configuration     | description
---------- | ----------------- | -----------
-s         |                   | acts as a cache-ing layer for another backend
-o         | sharedDir         | shares a system path.
-z         | zipfile           | uses a zip file's contents. Read only.

Mounting:

Mount the NFS path as you would normally. For the first example:

	mount 127.0.0.1:/ /mnt/point

Limitations:

This is a horrible hack by a someone who doesn't know much Go and knows even less C.
Thus there are obviously some limitations, most of which are probably unknown.
Of the known:

-In some (many? most?) systems, the server fails at start with an error along the
lines of "RPC: Authentication error; why = Client credential too weak". It's some
weird thing with rpcbind, and as a casual linux user I'm not sure what to do about
this. So far, the only solutions I've found are running unfs2go as root/sudo or
running rpcbind in insecure mode (-i). If anyone has any further information, I'd
greatly appreciate it.

Development:

You can develop your own backends if you want (If you make anything interesting,
let me know. I'll probably include it into main). Backends are based on the minfs.MinFS
interface. minfs.MinFS is a filesystem abstraction inspired by the afero.Fs interface,
(from the afero project https://github.com/spf13/afero ), as well as the vfs.FileSystem
interface (from golang's tool godoc https://go.googlesource.com/tools/+/master/godoc/vfs ).
However, minfs.MinFS attempts to be more succinct and faster to develop for than
afero.Fs, while being more full-featured than vfs.FileSystem. As a result, it's
not as much a drop-in-replacement for other programs besides unfs2go.

License Stuff:

In the "unfs3" folder you'll find the code that's been re-purposed from the UNFS3
project ( http://unfs3.sourceforge.net/ ), as well as the CREDITS and LICENSE files
for that code.

In the "zipfs" folder you'll find the code that's been re-purposed from the godoc
tool (https://go.googlesource.com/tools/+/master/godoc/vfs), as well as the AUTHORS,
LICENSE, etc. files for that code.

As for my paltry code and modifications:

UNFS 2 Go
(C) 2014, Zilog8 <zeuscoding@gmail.com>

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.
3. The name of the author may not be used to endorse or promote products
   derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS OR IMPLIED
WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO
EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS;
OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR
OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF
ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
