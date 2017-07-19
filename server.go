package main

import(
    "fmt"
    //"time"
    //"os"
    "strconv"
    "./idGenerator"
    "github.com/gin-gonic/gin"
)

var idWorkerMap = make(map[int]*idGenerator.IdWorker)

func main() {

    r := gin.Default()

    r.GET("/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "message": "pong",
        })
    })

    // Get ID
    r.GET("/worker/:id", func(c *gin.Context) {
        id, _ := strconv.Atoi(c.Params.ByName("id"))
        value, ok := idWorkerMap[id]
        if ok {
            nid, _ := value.NextId()
            c.JSON(200, gin.H{"id": nid})
        } else {
            iw, err := idGenerator.NewIdWorker(int64(id))
            if err == nil {
                nid, _ := iw.NextId()
                idWorkerMap[id] = iw
                c.JSON(200, gin.H{"id": nid})
            } else {
                fmt.Println(err)
            }
        }
    })

    // Listen and Server in 0.0.0.0:8182
    //r.Run(":8182")

    r.Run() // listen and serve on 0.0.0.0:8080
}
