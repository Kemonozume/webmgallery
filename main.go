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
		if strings.Contains(a.(string), e) {
			return true
		}
	}
	return false
}

func contains2(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(a, e) {
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
	ctx.Store.Init("leveldb", "test")

	e := echo.New()
	e.SetDebug(true)
	e.Use(mw.Recover())
	e.Static("/assets/", "public/assets")
	e.Get("/webms/:name", func(c *echo.Context) error {
		resp := c.Response()
		name := c.Param("name")
		resp.Header().Add("Content-Type", "video/webm")
		http.ServeFile(resp, c.Request(), "webms/"+name)
		return nil
	})
	e.Static("/webms/", "webms")

	//e.ServeFile("/", "public/index.html")
	e.Get("/", func(c *echo.Context) error {
		http.ServeFile(c.Response(), c.Request(), "public/index.html")
		return nil
	})

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

	e.Get("/webm/filter/:filter", func(c *echo.Context) error {
		tags := strings.Split(c.Param("filter"), "+")
		result, err := api.NewQuery("WebmCont", rootid).Collect("Webm").Run(ctx)
		if err != nil {
			return err
		}
		by, err := filter(result, tags...)
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

	e.Get("/webm/:id/tag/:tag", func(c *echo.Context) error {
		result, err := api.NewQuery("Webm", c.Param("id")).Run(ctx)
		if err != nil {
			return err
		}
		var tags []interface{}
		tagb, ok := result.Columns["tags"].Value.([]interface{})
		if ok {
			tags = tagb
		}
		cont := contains(tags, c.Param("tag"))
		var atags []string
		var resp string
		if cont {
			resp = "removed"
			atags = convertButLeave(tags, c.Param("tag"))
		} else {
			resp = "added"
			atags = convert(tags)
			atags = append(atags, c.Param("tag"))
		}
		err = api.Get("Webm", c.Param("id")).SetSource(rootid).Set("tags", atags).Execute(ctx)
		if err != nil {
			return err
		}

		return c.String(200, resp)
	})

	graceful.ListenAndServe(e.Server(":8080"), 5*time.Second)
}

func convertButLeave(s []interface{}, e string) (b []string) {
	for _, v := range s {
		if v.(string) != e {
			b = append(b, v.(string))
		}
	}
	return
}

func convert(s []interface{}) (b []string) {
	for _, v := range s {
		b = append(b, v.(string))
	}
	return
}
