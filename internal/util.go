package internal

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"os"
	"path"
	"path/filepath"
	"time"
)

// 文件处理

// IsEmptyDir 判断目录是否为空
func IsEmptyDir(dirPath string) (bool, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer dir.Close()
	//如果目录不为空，Readdirnames 会返回至少一个文件名
	_, err = dir.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func GetWorkPath() string {
	workDir, _ := os.Getwd()
	return workDir
}
func GetProcessPath() string {
	// 添加运行目录
	process, _ := os.Executable()
	return filepath.Dir(process)
}

func IsExistFile(filePath string) (os.FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err == nil {
		return stat, nil
	}
	if os.IsNotExist(err) {
		return nil, nil
	}
	return nil, err
}

// MD5

func Md5HashStr(str string) string {
	// 计算JSON数据的MD5
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}

// GetFileMd5 计算本地文件的MD5值
func GetFileMd5(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := md5.New()
	buffer := make([]byte, 1024*1024) // 1MB buffer
	_, err = io.CopyBuffer(hasher, file, buffer)
	if err != nil {
		return "", err
	}
	md5Bytes := hasher.Sum(nil)
	md5Str := hex.EncodeToString(md5Bytes)
	return md5Str, nil
}

// SHA1
// GetFileSha1 计算本地文件的SHA1值
func GetFileSha1(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha1.New()
	buffer := make([]byte, 1024*1024) // 1MB buffer
	_, err = io.CopyBuffer(hasher, file, buffer)
	if err != nil {
		return "", err
	}
	sha1Bytes := hasher.Sum(nil)
	sha1Str := hex.EncodeToString(sha1Bytes)
	return sha1Str, nil
}

// GetFileGcid 计算本地文件的Gcid值
func GetFileGcid(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}
	hasher := NewGcid(stat.Size())
	buffer := make([]byte, 1024*1024) // 1MB buffer
	_, err = io.CopyBuffer(hasher, file, buffer)
	if err != nil {
		return "", err
	}
	gcidBytes := hasher.Sum(nil)
	gcidStr := hex.EncodeToString(gcidBytes)
	return gcidStr, nil
}

var extraMimeTypes = map[string]string{
	".apk": "application/vnd.android.package-archive",
}

func GetMimeType(name string) string {
	ext := path.Ext(name)
	if m, ok := extraMimeTypes[ext]; ok {
		return m
	}
	m := mime.TypeByExtension(ext)
	if m != "" {
		return m
	}
	return "application/octet-stream"
}

// Log

func LogProgress(prefix, fileName string, startTime time.Time, thisOperated, operated, totalSize int64, mustLog bool) {
	elapsed := time.Since(startTime).Seconds()
	var speed float64
	if elapsed == 0 {
		speed = float64(thisOperated) / 1024
	} else {
		speed = float64(thisOperated) / 1024 / elapsed // KB/s
	}

	// 计算进度百分比
	percent := float64(operated) / float64(totalSize) * 100
	msg := fmt.Sprintf("%s %s: %.2f%% (%d/%d bytes, %.2f KB/s)", prefix, fileName, percent, operated, totalSize, speed)
	GetLogger().Debug(msg)
	if mustLog {
		GetLogger().Info(msg)
	}
	if operated == totalSize {
		if elapsed == 0 {
			speed = float64(operated) / 1024
		} else {
			speed = float64(operated) / 1024 / elapsed
		}
		GetLogger().Info(fmt.Sprintf("%s %s: %.2f%% (%d/%d bytes, %.2f KB/s), cost %.2f s", prefix, fileName, percent, operated, totalSize, speed, elapsed))
	}
}

// GenRandomWord 生成一个4位随机字谜
func GenRandomWord() string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 4)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
