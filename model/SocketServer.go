package model

import (
	"net"
	"time"
	"io"
	"fmt"
	"errors"
	"bytes"
	"encoding/binary"
	"bufio"
	"container/list"
	"idGenerator/model/logger"
	"math"
	"sync"
	"os"
	"path"
	"encoding/json"
	"strings"
	"net/rpc"
	"encoding/gob"
)

//var	contextList *list.List

const (
	STATUS_NULL = 0x00
	STATUS_NEW = 0x01
	STATUS_FINISH = 0xFF

	TIME_FORMAT = "2006-01-02 15:04:05"

	//server 类型
	SERVER_TYPE_DATA_BACKUP = 1 //数据备份server
	SERVER_TYPE_RPC = 2 //rcp server

	//server 的状态
	SERVER_STATUS_ALIVE int8 = 1
	SERVER_STATUS_DEAD int8 = 9
)

type MasterServer struct{
	ContextList *list.List
	ServerAddress string
	ServerType int
	ServerStatus int8
	WaitGroup sync.WaitGroup
	NetListener net.Listener
}

//var masterServer *MasterServer

func NewServer(serverAddress string, serverType int) *MasterServer {

	switch serverType {
	case SERVER_TYPE_DATA_BACKUP, SERVER_TYPE_RPC:
	default:
		panic("不识别的server type")
	}

	var wg sync.WaitGroup

	masterServer := &MasterServer{list.New(), serverAddress, serverType, SERVER_STATUS_ALIVE, wg, nil}

	return masterServer
}

func (masterServer *MasterServer) ToString() string {
	return fmt.Sprintf("server, address:%s, serverType:%d, serverStatus:%d", masterServer.ServerAddress, masterServer.ServerType, masterServer.ServerStatus)
}

//启动master server
func (masterServer *MasterServer) StartMasterServer() {

	serverAddress := masterServer.ServerAddress

	_, err := net.ResolveTCPAddr("tcp", serverAddress)
	CheckErr(err)

	listener, err := net.Listen("tcp", serverAddress)
	CheckErr(err)


	defer func() {
		masterServer.ServerStatus = SERVER_STATUS_DEAD //server 挂了
		listener.Close()
		//这里没有 recover
	}()

	logger.AsyncInfo(fmt.Sprintf("start master server:%s" , masterServer.ToString()))

	// 开启一个子 grountine 来遍历 contextList
	go masterServer.doConnectionAliveCheck()

	for {
		connection, err := listener.Accept()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		if masterServer.isDead() {
			break
		}

		now := time.Now().Unix()
		lock := new(sync.Mutex)
		var context = &Context{connection, now, lock,nil,nil}

		logger.AsyncInfo(fmt.Sprintf("new connection: %#v", connection))

		masterServer.ContextList.PushBack(context) //放入全局context list中

		switch masterServer.ServerType {
		case SERVER_TYPE_DATA_BACKUP:
			go masterServer.handleDataBackupConnection(context)
		case SERVER_TYPE_RPC:
			go masterServer.handleRpcConnection(context)
		default:
			panic("server type 异常")
		}

		masterServer.WaitGroup.Add(1)
	}

	masterServer.WaitGroup.Wait()
}

//RPC 服务处理
func (masterServer *MasterServer) handleRpcConnection(context *Context) {
	//解码器
	buf := bufio.NewWriter(context.Connection)
	codec := &GobServerCodec{
		rwc:    context,
		dec:    gob.NewDecoder(context.Connection),
		enc:    gob.NewEncoder(buf),
		encBuf: buf,
	}

	defer func() {
		codec.Close()
		masterServer.WaitGroup.Done()

		err := recover()

		if err != nil {
			logger.AsyncInfo(fmt.Sprintf("handleRpcConnection error:%#v", err))
		}
	}()

	//master 下发退出程序
	go func() {
		for {
			time.Sleep(2 * time.Second)
			if masterServer.isDead() {
				codec.Close()
				break
			}
		}
	}()

	//处理rpc 业务
	rpcServer := rpc.NewServer()
	rpcServer.Register(NewBoltDbRpcService()) //注册rpc 服务
	rpcServer.ServeCodec(codec)

}

//连接活跃情况检查
func (masterServer *MasterServer) doConnectionAliveCheck() {
	for {
		maxUnActiveTs := int64(math.Max(float64(GetApplication().ConfigData.MaxUnActiveTs), 10.0))

		for item := masterServer.ContextList.Front(); item != nil; item = item.Next() {
			context, ok := item.Value.(*Context)
			if !ok {
				masterServer.ContextList.Remove(item)
			}

			now := time.Now().Unix()

			if masterServer.isDead() {
				context.Connection.Close()
				masterServer.ContextList.Remove(item)

				logger.AsyncInfo(fmt.Sprintf("Server宕机关闭连接, now:%#v, connection%#v", now, context))
			}

			if now - context.LastActiveTs > maxUnActiveTs {
				context.Connection.Close()
				logger.AsyncInfo(fmt.Sprintf("超时关闭连接, now:%#v, connection%#v", now, context))
				masterServer.ContextList.Remove(item)
			}
		}

		if masterServer.isDead() {
			logger.AsyncInfo("master server 宕机")
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func (masterServer *MasterServer) handleDataBackupConnection(context *Context) {
	defer func() {
		context.Connection.Close()
		masterServer.WaitGroup.Done() //子goroutine 退出

		err := recover()
		if err != nil {
			logger.AsyncInfo(fmt.Sprintf("handleDataBackupConnection error:%#v", err))
		}
	}()

	for {

		dataPackage := GetDecodedPackageData(context.getReader(), context.Connection)
		context.LastActiveTs = time.Now().Unix()

		masterServer.handleAction(context, dataPackage)

		if masterServer.isDead() {
			panic("server is dead")
		}
	}
}

func (masterServer *MasterServer) handleAction(context *Context, dataPacakge *BackupPackage) {

	if dataPacakge.DataLength < 0 {
		panic("数据包长度少于0")
	}

	//logger.AsyncInfo("开始处理请求" + fmt.Sprintf("dataPacakge:%#v", dataPacakge))

	var sendChunkEnd bool = false

	switch dataPacakge.ActionType {
	case ACTION_PING:
		dataPackage  := NewBackupPackage(ACTION_PING)
		dataPackage.encodeData(int32ToBytes(int(context.LastActiveTs)))
		_, err :=context.writePackage(dataPackage)
		//logger.AsyncInfo(fmt.Sprintf("心跳包: %#v, %#v", n, err))
		checkErr(err)
		break

	case ACTION_SYNC_DATA:

		var slaveFileInfo map[string]string
		json.Unmarshal(dataPacakge.Data, &slaveFileInfo)

		caculatedMd5 := CaculteFileMd5(GetApplication().ConfigData.Bolt.FilePath)

		if strings.Compare(slaveFileInfo["md5"],caculatedMd5) == 0 {
			logger.AsyncInfo("数据无修改，无需备份\t" + time.Now().Format(TIME_FORMAT) )
			sendChunkEnd = true
			break
		}

		logger.AsyncInfo("开始备份数据\t" + time.Now().Format(TIME_FORMAT) )
		//logger.AsyncInfo(slaveFileInfo)
		//logger.AsyncInfo("master md5值:\t" + caculatedMd5)

		// start 复制临时文件
		srcFile, err := os.Open(GetApplication().ConfigData.Bolt.FilePath)
		defer srcFile.Close()
		checkErr(err)
		destFilePath := path.Join(path.Dir(GetApplication().ConfigData.Bolt.FilePath), fmt.Sprintf("%d_%s_%s", os.Getpid(), MyMd5(context.Connection.RemoteAddr()), time.Now().Format("2006010215")))
		logger.AsyncInfo("临时文件路径:" + destFilePath)
		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE, 0644)
		defer os.Remove(destFilePath) //同步完成删除临时文件

		_, err = io.Copy(destFile, srcFile)
		checkErr(err)
		destFile.Close()

		//end 复制临时文件

		destFile, err = os.Open(destFilePath)
		defer destFile.Close()
		checkErr(err)

		buffer := make([]byte, 1024)
		var isChunk bool = false
		var totalBytes int64 = 0;

		//for i:=1;i < 3;i++ {
		//	dataPackage := NewBackupPackage(ACTION_SYNC_DATA)
		//	dataPackage.encodeData([]byte(strconv.Itoa(i)))
		//	_, err = context.writePackage(dataPackage)
		//	checkErr(err)
		//}

		for {
			n, err := destFile.Read(buffer)
			if n <= 0 || (err != nil  && err != io.EOF) {
				if err != io.EOF {
					logger.AsyncInfo(fmt.Sprintf("读文件内容异常, %d,  %#v", n, err))
				}

				sendChunkEnd = true



				break
			}

			var dataPackage *BackupPackage

			if isChunk {
				dataPackage = NewBackupPackage(ACTION_CHUNK_DATA)
				//dataPackage = NewBackupPackage(ACTION_SYNC_DATA)
			} else {
				dataPackage = NewBackupPackage(ACTION_SYNC_DATA)
				isChunk = true
			}

			dataPackage.encodeData(buffer[0:n])
			logger.AsyncInfo(fmt.Sprintf("同步包, action:%#v, length:%d", dataPackage.ActionType, dataPackage.DataLength))
			//logger.AsyncInfo(fmt.Sprintf("同步包, %#v", dataPackage))
			//if dataPackage.ActionType == ACTION_SYNC_DATA {
			//	logger.AsyncInfo(dataPackage)
			//}

			_, err = context.writePackage(dataPackage)
			checkErr(err)

			totalBytes += int64(n)

			//time.Sleep(1 * time.Second)

			if n < 1024 {
				break
			}
		}
		logger.AsyncInfo(fmt.Sprintf("end备份数据\t%#v, total size: %#v", time.Now().Format(TIME_FORMAT), totalBytes))
		break

	default:
		logger.AsyncInfo("不识别的action")
	}

	if sendChunkEnd {
		//同步完成的 tag 包
		chunkEndPackage := NewBackupPackage(ACTION_CHUNK_END)
		chunkEndPackage.encodeData([]byte{0x1})
		_, err := context.writePackage(chunkEndPackage)
		checkErr(err)
	}

	//logger.AsyncInfo("end 处理请求")

	return
}

func (masterServer *MasterServer) isDead() bool {
	if masterServer.ServerStatus == SERVER_STATUS_DEAD {
		return true
	}

	return false
}

//获取数据包的长度
func getDataLength(socketio *bufio.ReadWriter) (int, error) {
	var byteSlice = make([]byte, 4)

	n, err := socketio.Read(byteSlice)
	if err != nil || n < 4 {
		return 0, errors.New("数据长度获取失败")
	}

	return int(bytesToInt32(byteSlice)), nil
}

//是否是可识别的action
func isNewAction(action byte) bool {
	if action == ACTION_PING || action == ACTION_SYNC_DATA {
		return true
	}

	return false
}

//整形转换成字节  
func int32ToBytes(n int) []byte {
    bytesBuffer := bytes.NewBuffer([]byte{})
	tmp := int32(n)
    binary.Write(bytesBuffer, binary.LittleEndian, tmp)
    return bytesBuffer.Bytes()
}

//字节转换成整形  
func bytesToInt32(b []byte) int32 {
    bytesBuffer := bytes.NewBuffer(b)
    var tmp int32
    binary.Read(bytesBuffer, binary.LittleEndian, &tmp)
    return tmp
}
