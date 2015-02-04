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
	"errors"
	"fmt"
"os"
)

func main() {

	args := os.Args[1:]
	
	tfs, err := parseArgs(args)

	if err != nil {
		fmt.Println("Error starting: ", err)
	}

	ns = tfs
		C.exports_parse(C.CString("/"), C.CString("rw"))
	C.start()
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
	return osfs.New(args[0])
}
