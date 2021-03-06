package model

import (
	"crypto/md5"
	"fmt"
	"os"
	"io"
)

func MyMd5(data interface{}) string {

	result := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%#v", data))))

	return result
}

//计算文件的 md5
func CaculteFileMd5(filePath string) string {

	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}

	defer file.Close()

	md5Hash := md5.New()

	// cclehui_todo 这里有问题需要改
	_ ,err = io.Copy(md5Hash, file)

	//if err != nil {
	//	panic("xxxxxxxxxx")
	//}
	//fmt.Printf("xxx%d, error:%#v, %x",n, err, md5Hash.Sum(nil))
	result := fmt.Sprintf("%x", md5Hash.Sum(nil))

	return result
}
