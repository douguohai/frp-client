package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/douguohai/frp-client2/message"
	"github.com/douguohai/frp-client2/utils"
	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/pkg/consts"
	"github.com/gorilla/mux"
)

var (
	adminPort, serverPort, frpAdminPort int

	server *client.Service

	// 服务器IP
	serverIp string = ""

	// 代理配置
	defaultProxyConfList  = map[string]config.ProxyConf{}
	activityProxyConfList = map[string]config.ProxyConf{}

	serverCfg = config.GetDefaultClientConf()

	ctx = context.Background()

	httpServerReadTimeout  = 60 * time.Second
	httpServerWriteTimeout = 60 * time.Second

	run int64 = 0
)

func init() {
	adminPort, _ = utils.GetAvailablePort()
	frpAdminPort, _ = utils.GetAvailablePort()
	//adminPort = 8080
	fmt.Println(adminPort)
	go startLocalServer(fmt.Sprintf(":%v", adminPort))
}

// dealMsg 处理消息
func dealMsg(msg message.Msg) interface{} {
	// 解析实际消息体 将interface转换为JSON字符串
	jsonData, err := json.Marshal(msg.Body)
	if err != nil {
		fmt.Println("转换为JSON时出错：", err)
		return message.Msg{
			Type: message.MsgParseErr,
		}
	}
	switch msg.Type {
	case message.SaveConfig:
		// 将interface转换为JSON字符串
		var server = message.ConnectServerMsg{}
		if err := json.Unmarshal(jsonData, &server); err != nil {
			fmt.Println("解析JSON时出错：", err)
			return message.Msg{
				Type: message.MsgParseErr,
			}
		} else {
			serverIp = server.ServerIp
			serverPort = server.ServerPort
			fmt.Println("收到配置信息 frp server：", serverIp, serverPort)
			return message.Msg{
				Type: message.Success,
			}
		}
	case "hello":
		return "pong"
	}
	return nil
}

// startLocalServer 开启本地服务
func startLocalServer(address string) error {

	router := mux.NewRouter()

	// 创建 CORS 处理函数
	corsHandler := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 设置允许的源
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("origin"))

			// 设置允许的方法
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")

			// 设置允许的请求头
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// 继续处理请求
			h.ServeHTTP(w, r)
		})
	}

	router.HandleFunc("/api/getProxy", func(writer http.ResponseWriter, request *http.Request) {
		var proxy = getProxy()
		fmt.Println("获取代理服务成功 frp server：", proxy)
		data := message.ProxyResult{
			Result: message.Result{
				Status: 0,
				Msg:    "操作成功",
			},
			Data: message.ProxyMsgVos{
				Items: proxy,
			},
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("GET")

	router.HandleFunc("/api/addProxy", func(writer http.ResponseWriter, request *http.Request) {
		// 读取请求体
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// 解析 JSON 数据
		var proxy = message.ProxyMsg{}
		err = json.Unmarshal(body, &proxy)
		if err != nil {
			http.Error(writer, "Failed to parse JSON", http.StatusBadRequest)
			return
		}

		if addProxy(proxy) != nil {
			http.Error(writer, "Failed to parse JSON", http.StatusBadRequest)
			return
		}

		fmt.Println("新增代理服务成功 frp server：", proxy)
		data := message.Result{
			Status: 0,
			Msg:    "操作成功",
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("POST")

	router.HandleFunc("/api/editProxy", func(writer http.ResponseWriter, request *http.Request) {
		// 读取请求体
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// 解析 JSON 数据
		var proxy = message.ProxyMsg{}
		err = json.Unmarshal(body, &proxy)
		if err != nil {
			http.Error(writer, "Failed to parse JSON", http.StatusBadRequest)
			return
		}

		if editProxy(proxy) != nil {
			http.Error(writer, "修改异常", http.StatusBadRequest)
			return
		}

		fmt.Println("修改代理服务成功 frp server：", proxy)
		data := message.Result{
			Status: 0,
			Msg:    "操作成功",
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("POST")

	router.HandleFunc("/api/delProxy", func(writer http.ResponseWriter, request *http.Request) {
		// 读取请求体
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// 解析 JSON 数据
		var proxy = message.ProxyMsg{}
		err = json.Unmarshal(body, &proxy)
		if err != nil {
			http.Error(writer, "Failed to parse JSON", http.StatusBadRequest)
			return
		}

		if delProxy(proxy) != nil {
			http.Error(writer, "删除异常", http.StatusBadRequest)
			return
		}

		fmt.Println("删除代理服务成功 frp server：", proxy)
		data := message.AjaxResult{
			ResponseStatus: 0,
			ResponseMsg:    "操作成功",
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("POST")

	router.HandleFunc("/api/openProxy", func(writer http.ResponseWriter, request *http.Request) {
		// 读取请求体
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// 解析 JSON 数据
		var proxy = message.ProxyStatus{}
		err = json.Unmarshal(body, &proxy)
		if err != nil {
			http.Error(writer, "Failed to parse JSON", http.StatusBadRequest)
			return
		}

		defer func() {
			if v := recover(); v != nil {
				print(v)
				buildFail(writer, "操作失败")
				return
			}
		}()

		err = openProxy(proxy)
		if err != nil {
			buildFail(writer, err.Error())
			return
		}

		fmt.Println("修改状态成功 frp server：", proxy)
		data := message.AjaxResult{
			ResponseStatus: 0,
			ResponseMsg:    "操作成功",
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("PUT")

	router.HandleFunc("/api/connect", func(writer http.ResponseWriter, request *http.Request) {
		if serverIp == "" || serverPort == 0 {
			data := message.AjaxResult{
				ResponseStatus: -1,
				ResponseMsg:    "请配置服务器并锁定配置",
			}
			jsonData, _ := json.Marshal(data)
			writer.Write(jsonData)
			return
		}
		go connectFrpServer()
		data := message.AjaxResult{
			ResponseStatus: 0,
			ResponseMsg:    "操作成功",
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("GET")

	router.HandleFunc("/api/unlock", func(writer http.ResponseWriter, request *http.Request) {
		if serverIp == "" || serverPort == 0 {
			data := message.AjaxResult{
				ResponseStatus: -1,
				ResponseMsg:    "请配置服务器并锁定配置",
			}
			jsonData, _ := json.Marshal(data)
			writer.Write(jsonData)
			return
		}
		unlockConfig()
		data := message.AjaxResult{
			ResponseStatus: 0,
			ResponseMsg:    "操作成功",
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("GET")

	// 静态资源路由
	staticDir := "./resources/dist/"
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	fmt.Println("[监听服务] 本地服务监听 %i 端口", adminPort)

	webServer := &http.Server{
		Addr:         address,
		Handler:      corsHandler(router),
		ReadTimeout:  httpServerReadTimeout,
		WriteTimeout: httpServerWriteTimeout,
	}

	if address == "" {
		address = ":http"
	}
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	go func() {
		_ = webServer.Serve(ln)
	}()
	return nil
}

// addProxy 添加代理
// proxy 代理信息
func addProxy(proxy message.ProxyMsg) (err error) {
	cfg := &config.TCPProxyConf{}
	cfg.ProxyName = proxy.ProxyName

	cfg.ProxyType = consts.TCPProxy
	cfg.LocalIP = "127.0.0.1"
	cfg.LocalPort = proxy.LocalPort
	cfg.RemotePort = proxy.RemotePort
	cfg.UseEncryption = false
	cfg.UseCompression = false
	cfg.BandwidthLimit, err = config.NewBandwidthQuantity("")
	if err != nil {
		fmt.Println(err)
		return
	}
	cfg.BandwidthLimitMode = config.BandwidthLimitModeClient

	err = cfg.ValidateForClient()
	if err != nil {
		fmt.Println(err)
		return
	}

	defaultProxyConfList[cfg.ProxyName] = cfg
	return nil
}

// getProxy 获取代理列表
func getProxy() []message.ProxyMsgVo {
	values := make([]message.ProxyMsgVo, 0, len(defaultProxyConfList))
	for _, value := range defaultProxyConfList {
		proxyType := strings.ToLower(value.GetBaseConfig().ProxyType)
		switch proxyType {
		case "tcp":
			temp := value.(*config.TCPProxyConf)
			_, exist := activityProxyConfList[temp.ProxyName]
			values = append(values, message.ProxyMsgVo{
				ProxyName:  temp.ProxyName,
				Type:       temp.ProxyType,
				LocalPort:  temp.LocalPort,
				RemotePort: temp.RemotePort,
				Status:     exist,
			})
		}
	}
	return values
}

// addProxy 添加代理
// proxy 代理信息
func editProxy(proxy message.ProxyMsg) (err error) {

	old, has := defaultProxyConfList[proxy.ProxyName]
	if !has {
		return errors.New("不存在该名称的代理")
	}

	tcpProxy := old.(*config.TCPProxyConf)

	tcpProxy.LocalPort = proxy.LocalPort
	tcpProxy.RemotePort = proxy.RemotePort

	err = tcpProxy.ValidateForClient()
	if err != nil {
		fmt.Println(err)
		return errors.New("核验配置错误")
	}
	defaultProxyConfList[tcpProxy.ProxyName] = tcpProxy
	return nil
}

// addProxy 添加代理
// proxy 代理信息
func delProxy(proxy message.ProxyMsg) (err error) {
	_, has := defaultProxyConfList[proxy.ProxyName]
	if !has {
		return errors.New("不存在该名称的代理")
	}
	delete(defaultProxyConfList, proxy.ProxyName)
	return nil
}

// addProxy 添加代理
// proxy 代理信息
func openProxy(proxyStatus message.ProxyStatus) error {
	if run == 0 {
		return errors.New("远程服务未连接，请连接远程服务")
	}
	proxy, has := defaultProxyConfList[proxyStatus.ProxyName]
	if !has {
		return errors.New("不存在该名称的代理")
	}
	if proxyStatus.Status {
		activityProxyConfList[proxyStatus.ProxyName] = proxy
	} else {
		delete(activityProxyConfList, proxyStatus.ProxyName)
	}
	return server.ReloadConf(activityProxyConfList, nil)
}

// connectFrpServer 连接frp服务器
func connectFrpServer() {
	if run == 1 {
		server.Close()
	}
	serverCfg.AdminUser = "admin"
	serverCfg.AdminPwd = "admin"
	serverCfg.DialServerTimeout = 3
	frpAdminPort, _ = utils.GetAvailablePort()
	serverCfg.AdminPort = frpAdminPort

	server, _ = client.NewService(serverCfg, activityProxyConfList, nil, "")

	serverCfg.ServerAddr = serverIp
	serverCfg.ServerPort = serverPort
	if err := serverCfg.Validate(); err != nil {
		err = fmt.Errorf("parse config error: %v", err)
		return
	}
	var err error
	server, err = client.NewService(serverCfg, activityProxyConfList, nil, "")
	atomic.CompareAndSwapInt64(&run, int64(0), int64(1))
	err = server.Run(ctx)
	if err != nil {
		fmt.Println(err)
		atomic.CompareAndSwapInt64(&run, int64(1), int64(0))
		activityProxyConfList = map[string]config.ProxyConf{}
		// w.SendMessage(message.Msg{
		// 	Type: message.ConnectError}, func(m *astilectron.EventMessage) {
		// })
	}
}

func unlockConfig() {
	serverCfg.ServerAddr = ""
	serverCfg.ServerPort = 0
	activityProxyConfList = map[string]config.ProxyConf{}
	if run == 1 {
		server.Close()
	}
}

func buildFail(writer http.ResponseWriter, msg string) {
	data := message.Result{
		Status: -1,
		Msg:    msg,
	}
	jsonData, _ := json.Marshal(data)
	writer.Write(jsonData)
	return
}
