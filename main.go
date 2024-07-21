package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type ArchiveFile struct {
	FullPath string
	Size int64
}

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Help: %s [apk file path] [resource base path]\n", os.Args[0])
		return
	}

	outputPath := "size-report.txt"

	logFile, err := os.OpenFile(outputPath, os.O_CREATE | os.O_TRUNC | os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}

	defer func(logFile *os.File) {
		err := logFile.Close()
		if err != nil {
			panic(err)
		}
	}(logFile)

	mw := io.MultiWriter(os.Stdout, logFile)
	//log.SetOutput(mw)

	rootSrc := os.Args[1] // "/Users/gb/Downloads/top.plusalpha.ripper-1.0.0.288.apk"
	rootSrc, err = filepath.Abs(rootSrc)
	if err != nil {
		panic(err)
	}

	// 압축 파일 풀 경로 임시로 만든다.
	rootDst, err := os.MkdirTemp("", filepath.Base(rootSrc))
	if err != nil {
		panic(err)
	}

	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			panic(err)
		}
	}(rootDst) // clean up

	err = unzip(rootDst, rootSrc)
	if err != nil {
		panic(err)
	}

	obbSrc := path.Join(rootDst, "assets/main.obb.png")
	obbDst := path.Join(rootDst, "assets/main.obb")

	err = unzip(obbDst, obbSrc)
	if err != nil {
		panic(err)
	}

	err = os.Remove(obbSrc)
	if err != nil {
		panic(err)
	}

	resourceBasePath := os.Args[2] // "assets/main.obb/Ripper/Content/Paks/Ripper-Android_ASTC"

	pakSrc := path.Join(rootDst, resourceBasePath + ".pak")
	pakDst := strings.TrimSuffix(pakSrc, filepath.Ext(pakSrc)) + "_pak"

	err = unpak(pakDst, pakSrc)
	if err != nil {
		panic(err)
	}

	err = os.Remove(pakSrc)
	if err != nil {
		panic(err)
	}

	ucasSrc := path.Join(rootDst, resourceBasePath + ".ucas")
	ucasDst := strings.TrimSuffix(ucasSrc, filepath.Ext(ucasSrc)) + "_ucas"

	err = unpak(ucasDst, ucasSrc)
	if err != nil {
		panic(err)
	}

	err = os.Remove(ucasSrc)
	if err != nil {
		panic(err)
	}

	var archiveFiles []ArchiveFile

	errWalk := filepath.WalkDir(rootDst, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				panic(err)
			}

			//println(path, info.Size())
			archiveFiles = append(archiveFiles, ArchiveFile{
				FullPath: path,
				Size:     info.Size(),
			})
		}
		return nil
	})

	if errWalk != nil {
		panic(errWalk)
	}

	sort.Slice(archiveFiles, func (i, j int) bool {
		return archiveFiles[i].Size > archiveFiles[j].Size
	})

	pathStartIndex := len(rootDst) + 1

	// 확장자별 합산 용량 출력
	extSizeMap := make(map[string]int64)

	for _, f := range archiveFiles {
		extSizeMap[filepath.Ext(f.FullPath)] += f.Size
	}

	_, _ = fmt.Fprintf(mw, "Size report for file %s\n", rootSrc)

	_, _ = fmt.Fprintln(mw, "")

	_, _ = fmt.Fprintln(mw, "==== Size by extensions ====")

	var archiveFileByExts []ArchiveFile

	for k := range extSizeMap {
		//_, _ = fmt.Fprintln(mw, byteCountIEC(extSizeMap[k]), k)
		archiveFileByExts = append(archiveFileByExts, ArchiveFile{
			FullPath: k,
			Size:     extSizeMap[k],
		})
	}

	sort.Slice(archiveFileByExts, func (i, j int) bool {
		return archiveFileByExts[i].Size > archiveFileByExts[j].Size
	})

	for _, f := range archiveFileByExts {
		_, _ = fmt.Fprintf(mw, "%10s\t%s\n", byteCountIEC(f.Size), f.FullPath)
	}

	_, _ = fmt.Fprintln(mw, "")

	_, _ = fmt.Fprintln(mw, "==== Size by files ====")

	// 파일별 용량 출력
	for _, f := range archiveFiles {
		_, _ = fmt.Fprintf(mw, "%10s\t%s\n", byteCountIEC(f.Size), f.FullPath[pathStartIndex:])
	}

	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Size report file created at %s\n", outputPath)
}

func byteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

func unpak(dst string, src string) error {
	unpakCmd := exec.Command("/Users/Shared/Epic Games/UE_5.4/Engine/Binaries/Mac/UnrealPak", src, "-extract", dst)
	output, err := unpakCmd.Output()
	if err != nil {
		return err
	}

	fmt.Println(string(output))
	return nil
}

func unzip(dst string, zipSrc string) error {
	err := os.RemoveAll(dst)
	if err != nil {
		panic(err)
	}

	archive, err := zip.OpenReader(zipSrc)

	if err != nil {
		panic(err)
	}

	defer func(archive *zip.ReadCloser) {
		err := archive.Close()
		if err != nil {
			panic(err)
		}
	}(archive)

	for _, f := range archive.File {
		filePath := filepath.Join(dst, f.Name)
		fmt.Println("unzipping file ", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(dst)+string(os.PathSeparator)) {
			fmt.Println("invalid file path")
			return errors.New("invalid file path")
		}
		if f.FileInfo().IsDir() {
			fmt.Println("creating directory...")
			err := os.MkdirAll(filePath, os.ModePerm)
			if err != nil {
				panic(err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			panic(err)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			panic(err)
		}

		fileInArchive, err := f.Open()
		if err != nil {
			panic(err)
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			panic(err)
		}

		err2 := dstFile.Close()
		if err != nil {
			panic(err2)
		}

		err3 := fileInArchive.Close()
		if err3 != nil {
			panic(err3)
		}
	}
	return nil
}
