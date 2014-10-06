UNFS2GO

My first "project" in CGO (and C for that matter);
A fork of UNFS3 (Usermode NFS3 server) that uses Go as a backend.

The sample implementation uses the "zipfs" package found in the godoc
tool's source code to serve a zip file's contents (read-only).

Why:
I have a server without FUSE and who's kernel I can't modify.
I noticed it could mount NFS, however, so I started wondering if
NFS could be used as a poor-man's FUSE? Since I couldn't find an NFS server
in Go to hack, I figured I'd make one myself. Upon seeing the complexity
of the NFS protocol, I figured my best bet was to re-purposed a
pre-existing usermode NFS server. The best option I could find was UNFS3,
which unfortunately is written in C. Since I didn't know C, I used CGO to
create this gruesome chimera of UNFS3 and a Go backend.

Around the same time, I was monkeying around with godoc's zipfs package
as a way of being able to serve up a whole website from a zip file, and
thought that it would make a perfect first use for UNFS2GO.

Build:
	go build unfs2go.go unfs2go_exports.go


Usage:
You'll probably need rcpbind and nfs-common packages on all machines involved.
In debian:
	sudo apt-get install rpcbind nfs-common
	
Let it be that a zip file "voynich_manuscript.zip" exists, whose contents are
inside a folder by the name of "vman"

	on server (ipaddress == 192.168.1.1):
	./unfs2go ./voynich_manuscript.zip

	on client:
	mount 192.168.1.1:/vman /mountpoint


Limitations:
This is a horrible hack by a someone who doesn't know much Go and even less C.
Thus there are obviously some limitations, most of which are probably unknown.
Of the known:
-There seems to be a limitation to zipfs that prevents mounting the "root" directory
of a zip file. Only subfolders can be mounted.


License Stuff:

In the "unfs3" folder you'll find the code that's been re-purposed from the UNFS3
project ( http://unfs3.sourceforge.net/ ), as well as the CREDITS and LICENSE files
for that code.

In the "vfs" folder you'll find the code that's been re-purposed from the godoc
tool, as well as the AUTHORS, LICENSE, etc. files for that code.

Also, you'll find here a .zip file ("voynich_manuscript.zip"), a thumbnail copy of the
Voynich Manuscript gotten from Archive.org, just for testing purposes. That is a
Public Domain work.

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
