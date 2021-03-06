package model

import (
	"errors"
	"fmt"
)

type BoltDbRpcService  struct {
	BoltDbService *BoltDbService
}

type LoadCurrentIdFromDbArgs struct {
	Source string
	BucketStep int
}

func (this *BoltDbRpcService) LoadCurrentIdFromDb(args *LoadCurrentIdFromDbArgs, result *int) (err error) {

	defer func() {
		errRecovered := recover()

		if errRecovered != nil {
			err = errors.New(fmt.Sprintf("%#v", errRecovered))
		}
	}()

	*result = this.BoltDbService.LoadCurrentIdFromDb(args.Source, args.BucketStep)
	return err
}

type IncrSourceCurrentIdArgs struct {
	Source string
	CurrentId int
	BucketStep int
}

type IncrSourceCurrentIdResult struct {
	ResultCurrentId	int
	NewDbCurrentId int
}

func (this *BoltDbRpcService) IncrSourceCurrentId(args *IncrSourceCurrentIdArgs, result *IncrSourceCurrentIdResult) (err error) {

	defer func() {
		errRecovered := recover()

		if errRecovered != nil {
			err = errors.New(fmt.Sprintf("%#v", errRecovered))
		}
	}()

	resultCurrentId, newDbCurrentId := this.BoltDbService.IncrSourceCurrentId(args.Source, args.CurrentId, args.BucketStep)

	result.ResultCurrentId = resultCurrentId
	result.NewDbCurrentId = newDbCurrentId

	return err
}

//保活 keep alive 请求
func (this *BoltDbRpcService) KeepAlive(args int, result *int) (err error) {
	*result = args + 1
	return nil
}

func NewBoltDbRpcService() *BoltDbRpcService {
	return &BoltDbRpcService{NewBoltDbService()}
}

/******************************************************/
type BoltDbRpcClient struct {
	Client *Client
}

func NewBoltDbRpcClient(socketClient *Client) *BoltDbRpcClient {
	if socketClient == nil {
		panic("rpc socket client 为 nil")
	}
	return &BoltDbRpcClient{socketClient}
}


func(this *BoltDbRpcClient) LoadCurrentIdFromDb(source string, bucketStep int) int {

	args := LoadCurrentIdFromDbArgs{Source:source, BucketStep:bucketStep}
	result := 0

	err := this.Client.GetRpcClient().Call("BoltDbRpcService.LoadCurrentIdFromDb", args, &result)
	CheckErr(err)

	return result
}

func(this *BoltDbRpcClient)  IncrSourceCurrentId(source string, currentId int, bucketStep int) (resultCurrentId int, newDbCurrentId int) {

	args := IncrSourceCurrentIdArgs{Source:source, CurrentId:currentId, BucketStep:bucketStep}
	result := new(IncrSourceCurrentIdResult)

	err := this.Client.GetRpcClient().Call("BoltDbRpcService.IncrSourceCurrentId", args, result)
	CheckErr(err)

	return result.ResultCurrentId, result.NewDbCurrentId
}
