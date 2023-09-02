package main

import (
	"embed"
	"fmt"
	"net/http"
	"regexp"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "内网穿刺",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

					router := getLocalServerRoute()

					// 匹配以/api/开头的任意路径
					apiPattern := regexp.MustCompile("^/api/.*$")

					if apiPattern.MatchString(r.URL.Path) {
						router.ServeHTTP(w, r)
						return
					}

					// 在这里实现你的中间件逻辑
					// 可以在请求被处理之前或之后执行一些处理
					fmt.Println(r.RequestURI)

					// 执行下一个中间件或处理函数
					next.ServeHTTP(w, r)
				})
			},
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.shutdown,
		Bind: []interface{}{
			app,
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: true,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
