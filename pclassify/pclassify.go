package main

import (
	"errors"
	"fmt"
	"github.com/rwcarlsen/goexif/exif"
	"os"
	"path/filepath"
	"photoutils/pcopy/pcopylib"
	"runtime"
	"strings"
	"time"
)

func shortUsage(errInfo string) error {
	str := fmt.Sprintln("usage: pclassify [-h] [-c] [-f] [-m | -y | -b | -d] sourcePath [destPath]")
	str += fmt.Sprint(errInfo)
	err := errors.New(str)
	return err
}

func longUsage() {
	fmt.Println("usage: pclassify [-h] [-c] [-f] [-m] [-y] [-b] sourcePath [destPath]")
	fmt.Println("")
	fmt.Println("positional arguments:")
	fmt.Println("  sourcePath   source path for photos to be classified")
	fmt.Println("  destPath     specify destination path for classified photos(use source")
	fmt.Println("               path by default)")
	fmt.Println("")
	fmt.Println("optional arguments:")
	fmt.Println("  -h, --help   show this help message and exit")
	fmt.Println("  -c           copy file(s) from source to target(move file(s) by defualt)")
	fmt.Println("  -f           use fullhash mode(more slower than default)")
	fmt.Println("")
	fmt.Println("  classify mode options:")
	fmt.Println("    -m         classify photos by month(default)")
	fmt.Println("    -y         classify photos by year")
	fmt.Println("    -b         classify photos by birthday")
	fmt.Println("    -d         classify photos by date")
}

type typeClassifyMode int

const (
	monthMode typeClassifyMode = iota
	yearMode
	birthdayMode
	dateMode
	unknown
)

var (
	copyMode     bool             = false
	fullHashMode bool             = false
	classifyMode typeClassifyMode = unknown
	source       string           = ""
	target       string           = ""
)

func parseArgs() error {
	remainder := []string{}
	invalidArg := []string{}

	classifyModeMap := map[string]typeClassifyMode{"-b": birthdayMode, "-m": monthMode, "-y": yearMode, "-d": dateMode}

	for idx, arg := range os.Args {
		if idx == 0 {
			continue
		}

		switch {
		case arg == "-h" || arg == "--help":
			longUsage()
			os.Exit(0)
		case arg == "-c":
			copyMode = true
		case arg == "-f":
			fullHashMode = true
		case arg == "-b" || arg == "-y" || arg == "-m" || arg == "-d":
			if classifyMode == unknown {
				classifyMode = classifyModeMap[arg]
			} else {
				for opt, mode := range classifyModeMap {
					if mode == classifyMode {
						return shortUsage(fmt.Sprintf("pclassify: error: options %s and %s are mutally exclusive", opt, arg))
					}
				}
			}
		case arg[:1] == "-":
			invalidArg = append(invalidArg, arg)
		default:
			remainder = append(remainder, arg)
		}
	}

	if len(remainder) > 2 {
		invalidArg = append(invalidArg, remainder[:len(remainder)-2]...)
	}

	if len(remainder) < 1 {
		return shortUsage(fmt.Sprint("pclassify: error: too few arguments"))
	}

	if len(invalidArg) > 0 {
		return shortUsage(fmt.Sprintf("pclassify: error: unrecognized arguments: %s", strings.Join(invalidArg, " ")))
	}

	source = remainder[0]
	if len(remainder) == 2 {
		target = remainder[1]
	}

	if classifyMode == unknown {
		classifyMode = monthMode
	}

	if len(target) == 0 {
		target = source
	}

	return nil
}

func getDateFromExif(file string) (error, time.Time) {
	f, err := os.Open(file)
	defer f.Close()

	if err != nil {
		return errors.New("pclassify: warning: read exif info failed"), time.Now()
	}

	x, err := exif.Decode(f)
	if err != nil {
		return errors.New("pclassify: warning: read exif info failed"), time.Now()
	}

	ts, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		return errors.New("pclassify: warning: read exif info failed"), time.Now()
	}

	const layout = "2006:01:02 15:04:05"
	t, err := time.ParseInLocation(layout, ts.StringVal(), time.Local)
	if err != nil {
		return errors.New("pclassify: warning: read exif info failed"), time.Now()
	}

	return nil, t
}

func getDateFromModifyTime(file string) (error, time.Time) {
	fi, err := os.Stat(file)
	if err != nil {
		return errors.New("pclassify: warning: get file MT_TIME failed"), time.Now()
	}

	return nil, fi.ModTime()
}

func makeFolderByMonth(target string, date time.Time) (string, error) {
	dateString := date.Format("2006-01")
	folderPath := filepath.Join(target, dateString)

	if pcopylib.IsFileExist(folderPath) != pcopylib.FileExistStatus_Directory {
		os.Mkdir(folderPath, os.ModePerm|os.ModeDir)
	}

	if pcopylib.IsFileExist(folderPath) != pcopylib.FileExistStatus_Directory {
		return "", errors.New("pclassify: error: make folder by month failed")
	}

	return folderPath, nil
}

func makeFolderByYear(target string, date time.Time) (string, error) {
	dateString := date.Format("2006")
	folderPath := filepath.Join(target, dateString)

	if pcopylib.IsFileExist(folderPath) != pcopylib.FileExistStatus_Directory {
		os.Mkdir(folderPath, os.ModePerm|os.ModeDir)
	}

	if pcopylib.IsFileExist(folderPath) != pcopylib.FileExistStatus_Directory {
		return "", errors.New("pclassify: error: make folder by month failed")
	}

	return folderPath, nil
}

func makeFolderByBirthday(target string, date time.Time, file string) (string, error) {
	birthday := time.Date(2011, 3, 16, 13, 12, 30, 0, time.Local)

	deltaYear := date.Year() - birthday.Year()
	deltaMonth := date.Month() - birthday.Month()

	monthAfterBirth := int(deltaYear)*12 + int(deltaMonth)
	if date.Day() >= 16 {
		monthAfterBirth += 1
	}

	if monthAfterBirth < 0 {
		return "", errors.New("pclassify: error: the date photo taken is earlier than birthday")
	}

	yearTag := monthAfterBirth / 12
	monthTag := monthAfterBirth % 12
	if monthTag == 0 {
		yearTag -= 1
		monthTag = 12
	}

	dateString := ""
	extName := strings.ToLower(filepath.Ext(file))
	switch {
	case extName == ".jpg" || extName == ".cr2":
		dateString = fmt.Sprintf("%d岁%d月照", yearTag, monthTag)
	case extName == ".mp4" || extName == ".mov" || extName == ".3gp":
		dateString = fmt.Sprintf("%d岁%d月视频", yearTag, monthTag)
	}

	folderPath := filepath.Join(target, dateString)

	if pcopylib.IsFileExist(folderPath) != pcopylib.FileExistStatus_Directory {
		os.Mkdir(folderPath, os.ModePerm|os.ModeDir)
	}

	if pcopylib.IsFileExist(folderPath) != pcopylib.FileExistStatus_Directory {
		return "", errors.New("pclassify: error: make folder by month failed")
	}

	return folderPath, nil
}

func makeFolderByDate(target string, date time.Time) (string, error) {
	dateString := date.Format("2006-01-02")
	folderPath := filepath.Join(target, dateString)

	if pcopylib.IsFileExist(folderPath) != pcopylib.FileExistStatus_Directory {
		os.Mkdir(folderPath, os.ModePerm|os.ModeDir)
	}

	if pcopylib.IsFileExist(folderPath) != pcopylib.FileExistStatus_Directory {
		return "", errors.New("pclassify: error: make folder by month failed")
	}

	return folderPath, nil
}

func classify(file, target string, copyMode, fullHashMode bool, classifyMode typeClassifyMode) error {
	err, date := getDateFromExif(file)
	if err != nil {
		err, date = getDateFromModifyTime(file)
	}

	if err != nil {
		return err
	}

	folderPath := ""
	switch classifyMode {
	case monthMode:
		folderPath, err = makeFolderByMonth(target, date)
	case yearMode:
		folderPath, err = makeFolderByYear(target, date)
	case birthdayMode:
		folderPath, err = makeFolderByBirthday(target, date, file)
	case dateMode:
		folderPath, err = makeFolderByDate(target, date)
	}

	if err != nil {
		return err
	}

	targetFile := filepath.Join(folderPath, filepath.Base(file))
	err = pcopylib.CopyFile(file, targetFile, !copyMode, fullHashMode)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if err := parseArgs(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if pcopylib.IsFileExist(source) != pcopylib.FileExistStatus_Directory {
		fmt.Println(shortUsage(fmt.Sprintf("pclassify: error: %s: No such directory", source)))
		os.Exit(1)
	}

	if pcopylib.IsFileExist(target) != pcopylib.FileExistStatus_Directory {
		fmt.Println(shortUsage(fmt.Sprintf("pclassify: error: %s: No such directory", target)))
		os.Exit(1)
	}

	jobsNum := 1
	if !copyMode {
		jobsNum = 20
	}

	classifyJob := make(chan string, jobsNum)
	classifyDone := make(chan struct{}, jobsNum)

	for i := 0; i < jobsNum; i++ {
		go func(classifyDone chan<- struct{}, classifyJob <-chan string) {
			for file := range classifyJob {
				classify(file, target, copyMode, fullHashMode, classifyMode)
			}

			classifyDone <- struct{}{}
		}(classifyDone, classifyJob)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if source == path {
			return nil
		}

		if info.IsDir() {
			return filepath.SkipDir
		}

		extName := strings.ToLower(filepath.Ext(path))
		if extName != ".jpg" && extName != ".cr2" && extName != ".mp4" && extName != ".mov" && extName != ".3gp" {
			return nil
		}

		classifyJob <- path

		return nil
	})

	close(classifyJob)

	for i := 0; i < jobsNum; i++ {
		<-classifyDone
	}
}
