package main

/*
#include <stdio.h>
#include "unfs3/daemon.h"
#include "unfs3/daemon.c"
*/
import "C"
import (
	"./vfs"
	"./vfs/zipfs"
	"archive/zip"
	"errors"
	"fmt"
"os"
)

func main() {

	fs = make(vfs.NameSpace)
	args := os.Args[1:]
	
	loadOne := false
	
	for len(args)>=4 {
		oneMorsel :=  args[:4]
		args = args[4:]
		
		tfs, err := parseArgs(oneMorsel[:3])

	if err != nil {
			fmt.Println("Error setting up backend", oneMorsel, ":", err)
		} else {		
			fs.Bind(oneMorsel[3], tfs, oneMorsel[2], vfs.BindReplace)
			loadOne = true
		}
	}
	
	if len(args)>0 {
		fmt.Println("Insufficient arguments:", args)
		fmt.Println("Examples:")
		fmt.Println("-z /zipfile /path/in/zip /nfs/path      //Exports a path in a zip file, to the specified NFS path.")
	}

	if loadOne {
		//TODO: Make this full-permission some day
		C.exports_parse(C.CString("/"), C.CString("ro"))
	C.start()
}
}

func parseArgs(args []string) (vfs.FileSystem, error) {
	switch args[0] {
	case "-z":
		return zipfsPrep(args[1:])
	default:
		return nil, errors.New("Not a recognized argument: " + args[0])
	}
}

func zipfsPrep(args []string) (vfs.FileSystem, error) {
	rc, err := zip.OpenReader(args[0])
	if err != nil {
		return nil, errors.New("Error opening zip for reading: " + err.Error())
	}
	zfs := zipfs.New(rc, args[0])
	
	//verify that the requested bind path exists in the zip file
	_, err = zfs.ReadDir(args[1])
	if err != nil {
		zfs.Close()
		return nil, errors.New("Error setting bind path in zip: " + err.Error())
	}	
	return zfs, nil
}
