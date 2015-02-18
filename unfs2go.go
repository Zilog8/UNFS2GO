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
	"./shimfs"
	"./zipfs"
	"errors"
	"fmt"
"os"
	"os/signal"
	"strconv"
)

var ns minfs.MinFS //filesystem being shared

func main() {

	args := os.Args[1:]
	
	tfs, err := parseArgs(args)

	if err != nil {
		fmt.Println("Error starting: ", err)
	}

	ns = tfs
		C.exports_parse(C.CString("/"), C.CString("rw"))

	//Handle Ctrl-C so we can quit nicely

	cc := make(chan os.Signal, 1)
	signal.Notify(cc, os.Interrupt)
	go func() {
		<-cc
		fmt.Println("Cleaning up, then quitting.")
		ns.Close()
		fmt.Println("Quitting.")
		os.Exit(1)
	}()

	C.start()
}

func parseArgs(args []string) (minfs.MinFS, error) {
	switch args[0] {
	case "-z":
		return zipfsPrep(args[1:])
	case "-o":
		return osfsPrep(args[1:])
	case "-s":
		return shimfsPrep(args[1:])
	default:
		return nil, errors.New("Not a recognized argument: " + args[0])
	}
}

func osfsPrep(args []string) (minfs.MinFS, error) {
	return osfs.New(args[0])
}

func zipfsPrep(args []string) (minfs.MinFS, error) {
	return zipfs.New(args[0])
}

func shimfsPrep(args []string) (minfs.MinFS, error) {
	tempFolder := args[0]
	cacheSize, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, err
	}

	sub, err := parseArgs(args[2:])
	if err != nil {
		return nil, err
	}
	return shimfs.New(tempFolder, int64(cacheSize*1024*1024), sub)
}
