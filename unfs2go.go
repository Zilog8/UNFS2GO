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
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Need arguments! Options:")
		fmt.Println("-z /path/to/zip                 Sets the specified zip archive as the backend.")
		return
	}

	tfs, err := parseArgs(os.Args[1:])

	if err != nil {
		fmt.Println("Error setting up backend:", err)
		return
	}

	fs = tfs
	C.start()
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
	if len(args) != 1 {
		return nil, errors.New("Inappropriate number of arguments. Need 1, got " + strconv.Itoa(len(args)))
	}

	rc, err := zip.OpenReader(args[0])
	if err != nil {
		return nil, errors.New("Error opening zip for reading: " + err.Error())
	}

	//TODO: Permit users to select paths other than "/", once we start using vfs.NameSpace
	C.exports_parse(C.CString("/"), C.CString("ro"))
	return zipfs.New(rc, args[0]), nil
}
