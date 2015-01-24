UNFS2GO

My first "project" in CGO (and C for that matter);
A fork of UNFS3 (Usermode NFS3 server) that uses Go backends.

The Go backends are abstracted through the vfs.NameSpace package found in
godoc. This permits multiple backends to be bound to the same virtual
filesystem. Backends implement the afero.Fs interface, from the afero
project, making it easier to swap between them, and for you to implement
your own backends as well. The first implemented backend is zip files
(read-only), using the "zipfs" package also from godoc.

Why:

I have a server without FUSE and whose kernel I can't modify.
It can mount NFS shares however, so I started wondering if NFS could be
used as a poor-man's FUSE?  Upon seeing the complexity of the NFS protocol,
I figured my best bet was to re-purposed a pre-existing NFS server.
Since I couldn't find an NFS server in Go, I settled on UNFS3, a usermode 
server written in C. Because of the simplicity of CGO, it actually wasn't
too difficult to bring this gruesome chimera into existence (despite my
utter lack of C knowledge). Around the same time, I was monkeying around
with godoc's zipfs package and thought that it would make a perfect first
use for UNFS2GO. Some time later, I found Afero and figured it'd fit in 
great.

Build:

	go build unfs2go.go unfs2go_exports.go

Dependencies:

You'll probably need the rcpbind and nfs-common packages.

In debian:

	sudo apt-get install rpcbind nfs-common
	
Usage:
Arguments are given in multiples of 4; each quartet representing a binding:

	1) bind type							Ex.:  -z
	
	2) 1st config							Ex.:  ./voynich_manuscript.zip
	
	3) 2nd config							Ex.:  /vman
	
	4) path to bind to in the NFS server	Ex.:  /voyzip
	
For example:

	unfs2go -z ./voynich_manuscript.zip /vman /voyzip
	
You can add multiple quartets at a time:

	unfs2go -z ./mydocs1.zip /docs /zip1 -z ./mydocs2.zip /docs2 /zip2
	
You can even place bindings inside other bindings:

	unfs2go -z ./mydocs1.zip /docs /zip1 -z ./mydocs2.zip /docs2 /zip1/zip2

Backends:

bind type  | 1st configuration | 2nd configuration | description
---------- | ----------------- | ----------------- | -----------
-z         | zipfile           | path/in/zip       | uses a zip file's contents. Read only.
-o         | ignored           | path/in/fs        | shares a system path. Full access.

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

In the "afero" folder you'll find the code that's been re-purposed from spf13's
afero project ( https://github.com/spf13/afero ), as well as the LICENSE file
for that code.

In the "unfs3" folder you'll find the code that's been re-purposed from the UNFS3
project ( http://unfs3.sourceforge.net/ ), as well as the CREDITS and LICENSE files
for that code.

In the "vfs" folder you'll find the code that's been re-purposed from the godoc
tool (https://go.googlesource.com/tools/+/master/godoc/vfs), as well as the AUTHORS,
LICENSE, etc. files for that code.

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
