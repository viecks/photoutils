package pcopylib

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type FileExistStatus int

const (
	FileExistStatus_File FileExistStatus = iota
	FileExistStatus_Directory
	FileExistStatus_NotExist
)

func IsFileExist(path string) FileExistStatus {
	fileinfo, err := os.Stat(path)
	switch {
	case err != nil:
		return FileExistStatus_NotExist
	case fileinfo.IsDir() == true:
		return FileExistStatus_Directory
	default:
		return FileExistStatus_File
	}

}

func doCopy(source, target string) error {
	fileinfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		targetFile.Close()
		return err
	}

	err = targetFile.Close()
	if err != nil {
		return err
	}

	os.Chmod(target, fileinfo.Mode())
	os.Chtimes(target, fileinfo.ModTime(), fileinfo.ModTime())
	return nil
}

func doCopyOrMove(source, target string, moveMode bool) error {
	if moveMode {
		os.Rename(source, target)
		fmt.Printf("%s -----> %s\n", source, target)
	} else {
		doCopy(source, target)
		fmt.Printf("%s +++++> %s\n", source, target)
	}
	return nil
}

func getFullHash(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer file.Close()

	md5Hash := md5.New()
	io.Copy(md5Hash, file)
	return fmt.Sprintf("%x", md5Hash.Sum(nil))
}

func getParticalHash(filename string, filesize int64) string {
	blockSize := int64(50 * 1024)
	blockOffsets := []int64{int64(0), (filesize - blockSize) / 3, 2 * (filesize - blockSize) / 3, filesize - blockSize}

	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer file.Close()

	md5Hash := md5.New()
	for _, offset := range blockOffsets {
		file.Seek(offset, 0)
		io.CopyN(md5Hash, file, blockSize)
	}

	return fmt.Sprintf("%x", md5Hash.Sum(nil))
}

func hasSameContent(source, target string, fullHashMode bool) bool {
	fiSource, _ := os.Stat(source)
	fiTarget, _ := os.Stat(target)

	srcSize := fiSource.Size()
	dstSize := fiTarget.Size()

	if srcSize != dstSize {
		return false
	}

	srcMD5 := ""
	dstMD5 := ""
	if !fullHashMode && srcSize > 500*1024 {
		srcMD5 = getParticalHash(source, srcSize)
		dstMD5 = getParticalHash(target, dstSize)
	} else {
		srcMD5 = getFullHash(source)
		dstMD5 = getFullHash(target)
	}

	return srcMD5 == dstMD5 && len(srcMD5) != 0 && len(dstMD5) != 0
}

func renameFile(target string, idx int) string {
	extName := filepath.Ext(target)
	newTarget := target[:len(target)-len(extName)] + "(" + strconv.Itoa(idx) + ")" + extName
	return newTarget
}

func CopyFileInternal(source, target string, moveMode, fullHashMode bool) error {
	if IsFileExist(target) == FileExistStatus_NotExist {
		doCopyOrMove(source, target, moveMode)
		return nil
	}

	renameIdx := 1
	newTarget := target
	for IsFileExist(newTarget) != FileExistStatus_NotExist && !hasSameContent(source, newTarget, fullHashMode) {
		newTarget = renameFile(target, renameIdx)
		renameIdx += 1
	}

	target = newTarget
	if IsFileExist(target) == FileExistStatus_NotExist {
		doCopyOrMove(source, target, moveMode)
	} else {
		if moveMode {
			os.Remove(source)
		}
		fmt.Printf("%s ====== %s, skipped\n", source, target)
	}

	return nil
}

func CopyFile(source, target string, moveMode, fullHashMode bool) error {
	if IsFileExist(target) == FileExistStatus_Directory {
		CopyFileInternal(source, filepath.Join(target, filepath.Base(source)), moveMode, fullHashMode)
	} else {
		targetPath := filepath.Dir(target)
		if len(targetPath) == 0 {
			targetPath = "./"
		}

		if IsFileExist(targetPath) != FileExistStatus_Directory {
			return errors.New(fmt.Sprintf("pcopy: error: %s/: No such file or directory", targetPath))
		}

		CopyFileInternal(source, target, moveMode, fullHashMode)
	}

	return nil
}

type fileEntry struct {
	path string
	info os.FileInfo
}

func CopyDirectory(source, target string, moveMode, fullHashMode, recursiveMode bool) error {
	if source == target {
		return errors.New(fmt.Sprint("pcopy: error: %s and %s are identical (not copied).", source, target))
	}

	targetStatus := IsFileExist(target)
	if targetStatus != FileExistStatus_Directory {
		return errors.New(fmt.Sprint("pcopy: error: ", target, ": Invalid target, a directory expected"))
	}

	jobNum := 1
	if moveMode {
		jobNum = 10
	}

	copyFileJobs := make(chan fileEntry, jobNum)
	copyDone := make(chan struct{}, jobNum)

	for i := 0; i < jobNum; i++ {
		go func(copyDone chan<- struct{}, target string, copyFileJobs <-chan fileEntry) {
			for job := range copyFileJobs {
				sourceFilePath := job.path
				targetFilePath := filepath.Join(target, job.path[len(source)+1:])
				err := CopyFile(sourceFilePath, targetFilePath, moveMode, fullHashMode)

				if err != nil {
					fmt.Printf("pcopy: error: %s: Copy %s failed, skiped", sourceFilePath)
				}
			}

			copyDone <- struct{}{}
		}(copyDone, target, copyFileJobs)
	}

	dirList := make([]string, 0, 100)

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if source == path {
				return nil
			}

			if !recursiveMode {
				return filepath.SkipDir
			}

			relativeSourceDirectory := path[len(source)+1:]
			targetDirectory := filepath.Join(target, relativeSourceDirectory)

			if IsFileExist(targetDirectory) == FileExistStatus_NotExist {
				os.MkdirAll(targetDirectory, os.ModePerm|os.ModeDir)
			}

			if IsFileExist(targetDirectory) != FileExistStatus_Directory {
				fmt.Printf("pcopy: error: %s: Directory can not be created, skiped", targetDirectory)
				return filepath.SkipDir
			}

			dirList = append(dirList, path)
		} else {
			copyFileJobs <- fileEntry{path, info}
		}
		return nil
	})

	close(copyFileJobs)

	for i := 0; i < jobNum; i++ {
		<-copyDone
	}

	if moveMode {
		sort.Sort(sort.Reverse(sort.StringSlice(dirList)))
		for _, dirToRemove := range dirList {
			os.Remove(dirToRemove)
		}
	}

	return nil
}
