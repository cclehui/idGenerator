package model

func CheckErr(err interface{}) {
	if err != nil {
		panic(err)
	}
}

func SocketIOErrCheck(err interface{}) {


}