package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
)

func getData(c *gin.Context) {
	name := c.Param("name")

	ia, err := GetIA(name)
	if err != nil {
		c.String(http.StatusBadGateway, fmt.Sprintf("error while getting data: %v", err))
		return
	}

	reply, err := ia.Feed()
	if err != nil {
		c.String(http.StatusBadGateway, fmt.Sprintf("error while creating rss feed: %v", err))
		return
	}

	c.Header("Content-Type", "application/rss+xml")
	c.String(http.StatusOK, reply)
}

func main() {
	router := gin.Default()
	store := persistence.NewInMemoryStore(persistence.FOREVER)

	router.GET("/get/:name", cache.CachePage(store, time.Hour*32, getData))
	router.Run(os.Args[1])
}
