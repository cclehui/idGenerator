package main

import (
    "fmt"
    "errors"
    "idGenerator/model/cmap"
);

type Application struct {
    idWorkerMap cmap.ConcurrentMap
}

var application Application;

func main() {
    //fmt.Println("xxxxxxxx");
    //fmt.Println(application);

	var err error

	if err := errors.New("aaaaaaaa"); err != nil {
		err = err
		fmt.Printf("1111\t%#v\n", err)
	}

	temp := recover()

	fmt.Println(err)
	fmt.Println(temp)
    //data, err := test();
    //fmt.Printf("data:%#v, error: %#v\n", data, err)
}

func test() (result int, err error) {

    defer func() {
        e := recover();

        if panicErr, ok := e.(error); ok {
            err = panicErr
            fmt.Printf("3333333333:%#v\n" , err)
        } else {
            //panic(e)
        }
    }()

    fmt.Println("11111111")
    panic(errors.New("eeeeeeeee"))
    fmt.Println("22222222")

    return 1, nil
}
