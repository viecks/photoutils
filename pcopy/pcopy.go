package main

import (
	"errors"
	"fmt"
	"os"
	"photoutils/pcopy/pcopylib"
	"runtime"
	"strings"
)

func shortUsage(errInfo string) error {
	str := fmt.Sprintln("usage: pcopy [-h] [-m] [-f] [-r] source target")
	str += fmt.Sprint(errInfo)
	err := errors.New(str)
	return err
}

func longUsage() {
	fmt.Println("usage: pcopy [-h] [-m] [-f] [-R] source target")
	fmt.Println("")
	fmt.Println("positional arguments:")
	fmt.Println("  source      source path for photos to be classified")
	fmt.Println("  target      target path for photos classified")
	fmt.Println("")
	fmt.Println("optional arguments:")
	fmt.Println("  -h, --help  show this help message and exit")
	fmt.Println("  -m          move file(s) from source to target(copy file(s) by default)")
	fmt.Println("  -f          use fullhash mode (more slower than default)")
	fmt.Println("  -r          recursive mode")
}

var (
	moveMode      bool   = false
	fullHashMode  bool   = false
	recursiveMode bool   = false
	source        string = ""
	target        string = ""
)

func parseArgs() error {
	remainder := []string{}
	invalidArg := []string{}

	for idx, arg := range os.Args {
		if idx == 0 {
			continue
		}

		switch {
		case arg == "-h" || arg == "--help":
			longUsage()
			os.Exit(0)
		case arg == "-m":
			moveMode = true
		case arg == "-f":
			fullHashMode = true
		case arg == "-r":
			recursiveMode = true
		case arg[:1] == "-":
			invalidArg = append(invalidArg, arg)
		default:
			remainder = append(remainder, arg)
		}
	}

	if len(remainder) > 2 {
		invalidArg = append(invalidArg, remainder[:len(remainder)-2]...)
	}

	if len(remainder) < 2 {
		return shortUsage(fmt.Sprint("pcopy: error: too few arguments"))
	}

	if len(invalidArg) > 0 {
		return shortUsage(fmt.Sprintf("pcopy: error: unrecognized arguments: %s", strings.Join(invalidArg, " ")))
	}

	source = remainder[0]
	target = remainder[1]

	return nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if err := parseArgs(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sourceStatus := pcopylib.IsFileExist(source)
	if sourceStatus == pcopylib.FileExistStatus_NotExist {
		fmt.Println(shortUsage(fmt.Sprintf("pcopy: error: %s: No such file or directory", source)))
		os.Exit(1)
	}

	if sourceStatus == pcopylib.FileExistStatus_File {
		if err := pcopylib.CopyFile(source, target, moveMode, fullHashMode); err != nil {
			fmt.Println(shortUsage(fmt.Sprint(err)))
			os.Exit(1)
		}
	} else {
		if err := pcopylib.CopyDirectory(source, target, moveMode, fullHashMode, recursiveMode); err != nil {
			fmt.Println(shortUsage(fmt.Sprint(err)))
			os.Exit(1)
		}
	}
}
