package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo"
	mw "github.com/labstack/echo/middleware"
	"github.com/manishrjain/gocrud/api"
	"github.com/manishrjain/gocrud/req"
	"github.com/manishrjain/gocrud/store"
	"github.com/manishrjain/gocrud/x"
	"github.com/tylerb/graceful"
)

type WebmCont struct {
	Id   string `json:"id,omitempty"`
	Webm []Webm `json:"webms,omitempty"`
}

type Webm struct {
	Id string `json:"id,omitempty"`
}

func contains(s []interface{}, e string) bool {
	for _, a := range s {
		if a.(string) == e {
			return true
		}
	}
	return false
}

func filter(result *api.Result, tags ...string) (by []byte, err error) {
	fmt.Printf("%v\n", tags)
	nResult := &api.Result{
		Id:      result.Id,
		Kind:    result.Kind,
		Columns: result.Columns,
	}
	for _, v := range result.Children {
		ctags, ok := v.Columns["tags"]
		filtered := true
		if ok {
			for _, tag := range tags {
				if !contains(ctags.Value.([]interface{}), tag) {
					filtered = false
				}
			}
		}
		if filtered {
			nResult.Children = append(nResult.Children, v)
		}

	}
	return nResult.ToJson()
}

const rootid = "uid_root"

var ctx *req.Context

func main() {
	rand.Seed(time.Now().UnixNano())
	fmt.Println("Running...")

	ctx = new(req.Context)

	l := new(store.Leveldb)
	l.SetBloomFilter(13)
	ctx.Store = l
	//ctx.Store.Init("leveldb", x.UniqueString(10))
	ctx.Store.Init("leveldb", "test")

	e := echo.New()
	e.SetDebug(true)
	e.Use(mw.Gzip())
	e.Use(mw.Recover())
	e.Static("/assets/", "public/assets")
	e.Static("/webms/", "webms")

	e.ServeFile("/", "public/index.html")

	e.ServeFile("/upload", "public/upload.html")
	e.Post("/upload", func(c *echo.Context) error {
		req := c.Request()

		name := req.FormValue("name")
		ttags := req.FormValue("tags")
		tags := strings.Split(ttags, " ")
		path := x.UniqueString(10)

		// Read files
		file := req.MultipartForm.File["file"]
		src, err := file[0].Open()
		if err != nil {
			return err
		}
		defer src.Close()

		// Destination file
		dst, err := os.Create("webms/" + path + ".webm")
		if err != nil {
			return err
		}
		defer dst.Close()

		if _, err = io.Copy(dst, src); err != nil {
			return err
		}

		if err = api.Get("WebmCont", rootid).SetSource(rootid).AddChild("Webm").Set("path", path).
			Set("tags", tags).Set("name", name).Execute(ctx); err != nil {
			return err
		}

		return c.String(http.StatusOK, "eyo uploaded file")
	})

	e.Get("/webm", func(c *echo.Context) error {
		result, err := api.NewQuery("WebmCont", rootid).Collect("Webm").Run(ctx)
		if err != nil {
			return err
		}
		by, err := result.ToJson()
		if err != nil {
			return err
		}
		return c.String(200, string(by))
	})

	e.Get("/webm/:id", func(c *echo.Context) error {
		result, err := api.NewQuery("Webm", c.Param("id")).Run(ctx)
		if err != nil {
			return err
		}
		by, err := result.ToJson()
		if err != nil {
			return err
		}
		return c.String(200, string(by))
	})

	graceful.ListenAndServe(e.Server(":8080"), 5*time.Second)
}
