package main

/*
#include <stdio.h>
#include "unfs3/daemon.h"
#include "unfs3/daemon.c"
*/
import "C"
import (
	"./minfs"
	"./osfs"
	"./vfs"
	//"archive/zip"
	"errors"
	"fmt"
"os"
)

func main() {

	tns := vfs.New()
	args := os.Args[1:]
	
	loadOne := false
	
	for len(args)>=4 {
		oneMorsel :=  args[:4]
		args = args[4:]
		
		tfs, err := parseArgs(oneMorsel[:3])

	if err != nil {
			fmt.Println("Error setting up backend", oneMorsel, ":", err)
		} else {		
			tns.Bind(oneMorsel[3], tfs, oneMorsel[2], vfs.BindReplace)
			loadOne = true
		}
	}
	
	if !loadOne || len(args) > 0 {
		fmt.Println("Insufficient arguments:", args)
		fmt.Println("Examples:")
		fmt.Println("-z /zipfile /path/in/zip /nfs/path      //Exports a path in a zip file, to the specified NFS path.")
		fmt.Println("-o . /path/in/fs /nfs/path      		 //Exports a path in the filesystem, to the specified NFS path.")
	}

	if loadOne {
		ns = tns
		C.exports_parse(C.CString("/"), C.CString("rw"))
	C.start()
}
}

func parseArgs(args []string) (minfs.MinFS, error) {
	switch args[0] {
	//case "-z":
	//	return zipfsPrep(args[1:])
	case "-o":
		return osfsPrep(args[1:])
	default:
		return nil, errors.New("Not a recognized argument: " + args[0])
	}
}

func osfsPrep(args []string) (minfs.MinFS, error) {
	return osfs.New("/")
}

//TODO: Test this again, after the new changes.
//func zipfsPrep(args []string) (afero.Fs, error) {
//	rc, err := zip.OpenReader(args[0])
//	if err != nil {
//		return nil, errors.New("Error opening zip for reading: " + err.Error())
//	}
//	zfs := zipfs.New(rc, args[0])
//
//	//TODO: reimplement this without ReadDir
//	//verify that the requested bind path exists in the zip file
//	/* _, err = zfs.ReadDir(args[1])
//	if err != nil {
//		zfs.Close()
//		return nil, errors.New("Error setting bind path in zip: " + err.Error())
//	}	 */
//	return zfs, nil
//}
