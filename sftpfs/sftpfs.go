package sftpfs

import (
	"../minfs"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"
)

type sftpFS struct {
	name       string
	serverpath string //Server path being exported
	sftpClient *sftp.Client
	sshClient  *ssh.Client
	closed     bool //false normally, true if closed
}

func New(user, pass, server, serverpath string, port int) (minfs.MinFS, error) {
	conn, cc, err := setup(user, pass, server, port)
	if err != nil {
		cc.Close()
		conn.Close()
		return nil, err
	}
	name := "sftpFS://" + user + "@" + server + ":" + fmt.Sprint(port) + serverpath
	return &sftpFS{name, serverpath, cc, conn, false}, nil
}

func setup(USER, PASS, HOST string, PORT int) (*ssh.Client, *sftp.Client, error) {
	var auths []ssh.AuthMethod
	if aconn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(aconn).Signers))
	}
	if len(PASS) != 0 {
		auths = append(auths, ssh.Password(PASS))
	}

	config := ssh.ClientConfig{
		User: USER,
		Auth: auths,
	}
	addr := fmt.Sprintf("%s:%d", HOST, PORT)
	conn, err := ssh.Dial("tcp", addr, &config)
	if err != nil {
		fmt.Println("unable to connect to [%s]: %v", addr, err)
		return nil, nil, err
	}

	c, err := sftp.NewClient(conn)
	if err != nil {
		fmt.Println("unable to start sftp subsytem: %v", err)
		return nil, nil, err
	}
	return conn, c, nil
}

func (f *sftpFS) translate(path string) string {
	path = pathpkg.Clean("/" + path)
	return pathpkg.Clean(filepath.Join(f.serverpath, path))
}

//Complying with mimfs.MinFS
func (f *sftpFS) Close() error {
	fmt.Println("sftpFS: Close")
	if f.closed {
		return os.ErrInvalid
	}
	f.closed = true
	f.sftpClient.Close()
	f.sshClient.Close()
	f.name += " (Closed)"
	return nil
}

func (f *sftpFS) CreateFile(name string) error {
	//fmt.Println("sftpFS: CreateFile:", name)
	if f.closed {
		return os.ErrInvalid
	}
	realname := f.translate(name)
	fil, err := f.sftpClient.Create(realname)
	if err != nil {
		return errorTranslate(err)
	}
	fil.Close()
	return nil
}

func (f *sftpFS) ReadDirectory(name string) ([]os.FileInfo, error) {
	//fmt.Println("sftpFS: ReadDirectory:", name)
	if f.closed {
		return nil, os.ErrInvalid
	}
	realname := f.translate(name)
	fi, err := f.sftpClient.ReadDir(realname)
	return fi, errorTranslate(err)
}

func (f *sftpFS) Stat(name string) (os.FileInfo, error) {
	//fmt.Println("sftpFS: Stat:", name)
	if f.closed {
		return nil, os.ErrInvalid
	}
	realname := f.translate(name)
	fi, err := f.sftpClient.Lstat(realname)
	if err != nil {
		return nil, errorTranslate(err)
	}
	return fi, nil
}

func (f *sftpFS) CreateDirectory(name string) error {
	//fmt.Println("sftpFS: CreateDirectory:", name)
	if f.closed {
		return os.ErrInvalid
	}
	realname := f.translate(name)
	return errorTranslate(f.sftpClient.Mkdir(realname))
}

func (f *sftpFS) SetAttribute(path string, attribute string, newvalue interface{}) error {
	//fmt.Println("sftpFS: SetAttr:", attribute, "for", path, "to", newvalue)
	if f.closed {
		return os.ErrInvalid
	}
	realname := f.translate(path)
	err := os.ErrInvalid
	switch attribute {
	case "modtime":
		err = f.sftpClient.Chtimes(realname, time.Now(), newvalue.(time.Time))
	case "mode":
		err = f.sftpClient.Chmod(realname, newvalue.(os.FileMode))
	case "size":
		err = f.sftpClient.Truncate(realname, newvalue.(int64))
	}
	return errorTranslate(err)
}

func (f *sftpFS) GetAttribute(path string, attribute string) (interface{}, error) {
	//fmt.Println("sftpFS: GetAttr:", attribute, "for", path)
	if f.closed {
		return nil, os.ErrInvalid
	}
	realname := f.translate(path)
	fi, err := f.Stat(realname)
	if err != nil {
		return nil, errorTranslate(err)
	}
	switch attribute {
	case "modtime":
		return fi.ModTime(), nil
	case "mode":
		return fi.Mode(), nil
	case "size":
		return fi.Size(), nil
	default:
		return nil, os.ErrInvalid
	}
}

func (f *sftpFS) Remove(name string) error {
	//fmt.Println("sftpFS: Remove:", name)
	if f.closed {
		return os.ErrInvalid
	}
	realname := f.translate(name)
	return errorTranslate(f.sftpClient.Remove(realname))
}

func (f *sftpFS) String() string {
	return f.name
}

func (f *sftpFS) Move(oldpath string, newpath string) error {
	//fmt.Println("sftpFS: Move:", oldpath, "to", newpath)
	if f.closed {
		return os.ErrInvalid
	}
	orname := f.translate(oldpath)
	nrname := f.translate(newpath)
	return errorTranslate(f.sftpClient.Rename(orname, nrname))
}

func (f *sftpFS) ReadFile(name string, b []byte, off int64) (int, error) {
	//fmt.Println("sftpFS: ReadFile:", name)
	if f.closed {
		return 0, os.ErrInvalid
	}
	realname := f.translate(name)
	fh, err := f.sftpClient.Open(realname)
	if err != nil {
		return 0, errorTranslate(err)
	}
	defer fh.Close()

	_, err = fh.Seek(off, os.SEEK_SET)
	if err != nil {
		return 0, errorTranslate(err)
	}
	retVal, err := fh.Read(b)
	return retVal, errorTranslate(err)
}

func (f *sftpFS) WriteFile(name string, b []byte, off int64) (int, error) {
	//fmt.Println("sftpFS: WriteFile:", name)
	if f.closed {
		return 0, os.ErrInvalid
	}
	realname := f.translate(name)
	fh, err := f.sftpClient.OpenFile(realname, os.O_WRONLY)
	if err != nil {
		return 0, errorTranslate(err)
	}
	defer fh.Close()

	_, err = fh.Seek(off, os.SEEK_SET)
	if err != nil {
		return 0, errorTranslate(err)
	}
	retVal, err := fh.Write(b)
	return retVal, errorTranslate(err)
}

func errorTranslate(sftperr error) error {
	switch {
	case sftperr == nil:
		return nil
	case strings.Contains(sftperr.Error(), "SSH_FX_NO_SUCH_FILE"):
		return os.ErrNotExist
	default:
		return sftperr
	}
}
