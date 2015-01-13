UNFS2GO

My first "project" in CGO (and C for that matter);
A fork of UNFS3 (Usermode NFS3 server) that uses Go backends.

The Go backends are abstracted through the vfs.NameSpace package found in
godoc. This permits multiple backends to be bound to the same virtual
filesystem. The first implemented backend is zip files, using the "zipfs"
package also from godoc. For the time being, it's all read-only.

Why:
I have a server without FUSE and whose kernel I can't modify.
It can mount NFS shares however, so I started wondering if NFS could be
used as a poor-man's FUSE? Since I couldn't find an NFS server in Go, and
upon seeing the complexity of the NFS protocol, I figured my best bet was
to re-purposed a pre-existing usermode NFS server. The best option I could
find was UNFS3, which unfortunately is written in C. Since I didn't know C,
I used CGO to create this chimera.

Around the same time, I was monkeying around with godoc's zipfs package and
thought that it would make a perfect first use for UNFS2GO.

Build:
	go build unfs2go.go unfs2go_exports.go

Dependencies:
You'll probably need the rcpbind and nfs-common packages.
In debian:
	sudo apt-get install rpcbind nfs-common
	
Usage:
Arguments are given in multiples of 4; each quartet representing a binding:

	1) bind type							Ex.:  -z
	
	2) config1								Ex.:  ./voynich_manuscript.zip
	
	3) config2								Ex.:  /vman
	
	4) path to bind to in the NFS server	Ex.:  /voyzip
	
For example:
	unfs2go -z ./voynich_manuscript.zip /vman /voyzip
	
You can add multiple quartets at a time:
	unfs2go -z ./mydocs1.zip /docs /zip1 -z ./mydocs2.zip /docs2 /zip2
	
You can even place bindings inside other bindings:
	unfs2go -z ./mydocs1.zip /docs /zip1 -z ./mydocs2.zip /docs2 /zip1/zip2

Backends:
bind type	1st configuration	2nd configuration	description
-z			zipfile 			path/in/zip			uses a zip file's contents

Mounting:
Mount the NFS path as you would normally. For the first example:
	mount 127.0.0.1:/voyzip /mnt/point

Limitations:
This is a horrible hack by a someone who doesn't know much Go and knows even less C.
Thus there are obviously some limitations, most of which are probably unknown.
Of the known:
-There seems to be a limitation to zipfs that prevents binding the "root"
directory of a zip file. For now, only non-root directories can be bound.
-In some (many? most?) systems, the server fails at start with an error along the
lines of "RPC: Authentication error; why = Client credential too weak". It's some
weird thing with rpcbind, and as a casual linux user I'm not sure what to do about
this. So far, the only solutions I've found are running unfs2go as root/sudo or
running rpcbind in insecure mode (-i). If anyone has any further information, I'd
greatly appreciate it.

License Stuff:

In the "unfs3" folder you'll find the code that's been re-purposed from the UNFS3
project ( http://unfs3.sourceforge.net/ ), as well as the CREDITS and LICENSE files
for that code.

In the "vfs" folder you'll find the code that's been re-purposed from the godoc
tool, as well as the AUTHORS, LICENSE, etc. files for that code.

Also, you'll find here a .zip file ("voynich_manuscript.zip"), a thumbnail copy of
the Voynich Manuscript gotten from Archive.org, just for testing purposes. That is
a Public Domain work.

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
