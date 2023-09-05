package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/douguohai/frp-client2/message"
	"github.com/douguohai/frp-client2/utils"
	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/client/proxy"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/pkg/consts"
	"github.com/gorilla/mux"
)

var (
	serverPort, frpAdminPort int

	server *client.Service

	// 服务器IP
	serverIp string = ""

	// 代理配置
	defaultProxyConfList  = map[string]config.ProxyConf{}
	activityProxyConfList = map[string]config.ProxyConf{}
	proxyRunStatus        = map[string]message.TCPProxyStatsu{}

	serverCfg = config.GetDefaultClientConf()

	ctx = context.Background()

	run int64 = 0

	// 创建定时任务，每秒执行一次
	ticker = time.NewTicker(time.Second * 5)
)

func init() {
	frpAdminPort, _ = utils.GetAvailablePort()
}

// getLocalServerRoute 开启本地服务
func getLocalServerRoute() *mux.Router {

	router := mux.NewRouter()

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
				Time:  time.Now().UnixNano(),
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
				buildFail(writer, "操作失败", "")
				return
			}
		}()

		err = openProxy(proxy)
		if err != nil {
			fmt.Print(err)
			buildFail(writer, err.Error(), struct {
				Status bool `json:"status"`
			}{Status: true})
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

		// 读取请求体
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// 解析 JSON 数据
		var serverInfo = message.ConnectServerMsg{}
		err = json.Unmarshal(body, &serverInfo)
		if err != nil {
			http.Error(writer, "Failed to parse JSON", http.StatusBadRequest)
			return
		}

		defer func() {
			if v := recover(); v != nil {
				print(v)
				buildFail(writer, "操作失败", "")
				return
			}
		}()
		serverIp = serverInfo.ServerIp
		serverPort = serverInfo.ServerPort

		// 创建一个通道
		ch := make(chan int)

		// 启动一个协程执行某个任务，并将通道传递给它
		go connectFrpServer(ch)

		data := message.ResultC{
			Result: message.Result{
				Status: 0,
				Msg:    "连接成功",
			},
		}

		// 等待协程的反馈消息，并在 5 秒钟的时间内超时
		select {
		case msg := <-ch:
			fmt.Println(msg)
			data.Result = message.Result{
				Status: -1,
				Msg:    "连接失败,请检查服务器配置信息",
			}
		case <-time.After(4 * time.Second):
			fmt.Println("5 秒未返回错误，默认认为启动成功")
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("POST")

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

	router.HandleFunc("/api/getServer", func(writer http.ResponseWriter, request *http.Request) {
		data := message.ServiceResult{
			Result: message.Result{
				Status: 0,
				Msg:    "操作成功",
			},
			Data: getServiceInfo(),
		}
		jsonData, _ := json.Marshal(data)
		writer.Write(jsonData)
	}).Methods("GET")

	return router
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
			_, run := activityProxyConfList[temp.ProxyName]
			proxyStatus, has := proxyRunStatus[temp.ProxyName]
			tempStatus := true
			if !run {
				tempStatus = false
			}
			tempRemoteAddress := "暂无"
			if has {
				tempRemoteAddress = proxyStatus.RemoteAddr
				if proxyStatus.Status == proxy.ProxyPhaseRunning {

				} else if proxyStatus.Status == proxy.ProxyPhaseClosed || proxyStatus.Status == proxy.ProxyPhaseStartErr {
					tempStatus = false
					tempRemoteAddress = "暂无"
				}
			}
			values = append(values, message.ProxyMsgVo{
				ProxyName:  temp.ProxyName,
				Type:       temp.ProxyType,
				LocalPort:  temp.LocalPort,
				RemotePort: temp.RemotePort,
				Status:     tempStatus,
				RemoteAddr: tempRemoteAddress,
			})
		}
	}

	//对value 进行排序
	sort.Slice(values, func(i, j int) bool {
		return values[i].ProxyName > values[j].ProxyName
	})
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
	//判断当前代理如果处于运行中，重新刷新配置
	_, has = activityProxyConfList[proxy.ProxyName]
	if has {
		activityProxyConfList[tcpProxy.ProxyName] = tcpProxy
		if run == 1 {
			server.ReloadConf(activityProxyConfList, nil)
		}
	}
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
	//判断当前代理如果处于运行中，重新刷新配置
	_, has = activityProxyConfList[proxy.ProxyName]
	if has {
		delete(activityProxyConfList, proxy.ProxyName)
		if run == 1 {
			server.ReloadConf(activityProxyConfList, nil)
		}
	}
	return nil
}

// addProxy 添加代理
// proxy 代理信息
func openProxy(proxyStatus message.ProxyStatus) error {
	proxy, has := defaultProxyConfList[proxyStatus.ProxyName]
	if !has {
		return errors.New("不存在该名称的代理")
	}
	if proxyStatus.Status {
		activityProxyConfList[proxyStatus.ProxyName] = proxy
	} else {
		delete(activityProxyConfList, proxyStatus.ProxyName)
	}
	if run == 0 {
		return errors.New("远程服务未连接，请连接远程服务")
	}
	return server.ReloadConf(activityProxyConfList, nil)
}

// connectFrpServer 连接frp服务器
func connectFrpServer(ch chan int) {
	if run == 1 {
		server.Close()
	}
	run = -1
	serverCfg.AdminUser = "admin"
	serverCfg.AdminPwd = "admin"
	serverCfg.DialServerTimeout = 3
	frpAdminPort, _ = utils.GetAvailablePort()
	serverCfg.AdminPort = frpAdminPort

	server, _ = client.NewService(serverCfg, activityProxyConfList, nil, "")

	serverCfg.ServerAddr = serverIp
	serverCfg.ServerPort = serverPort
	if err := serverCfg.Validate(); err != nil {
		fmt.Print(err)
		return
	}
	var err error
	server, _ = client.NewService(serverCfg, activityProxyConfList, nil, "")
	atomic.CompareAndSwapInt64(&run, int64(-1), int64(1))
	err = server.Run(ctx)
	if err != nil {
		ch <- -1
		fmt.Println(err)
		atomic.CompareAndSwapInt64(&run, int64(1), int64(0))
		activityProxyConfList = map[string]config.ProxyConf{}
		ticker.Stop()
		// w.SendMessage(message.Msg{
		// 	Type: message.ConnectError}, func(m *astilectron.EventMessage) {
		// })
	}
}

// unlockConfig 解锁frp服务器配置
func unlockConfig() {
	serverCfg.ServerAddr = ""
	serverCfg.ServerPort = 0
	activityProxyConfList = map[string]config.ProxyConf{}
	if run == 1 {
		server.Close()
		atomic.CompareAndSwapInt64(&run, int64(1), int64(0))
	} else {
		atomic.CompareAndSwapInt64(&run, int64(-1), int64(0))
	}
}

// buildFail 构建失败
func buildFail(writer http.ResponseWriter, msg string, data interface{}) {
	temp := message.ResultC{
		Result: message.Result{
			Status: -1,
			Msg:    msg,
		},
		Data: data,
	}
	jsonData, _ := json.Marshal(temp)
	writer.Write(jsonData)
}

// tryGetProxyManager 尝试获取代理管理器
func doCron() {
	for range ticker.C {
		fmt.Println("定时任务执行", time.Now().Local())
		if run == 1 {
			getProxyStatus()
		} else {
			proxyRunStatus = map[string]message.TCPProxyStatsu{}
		}
	}
}

// tryGetProxyManager 尝试获取代理管理器
func getProxyStatus() {

	// 要执行的任务
	fmt.Println("定时任务执行")

	//获取service 中的ctl属性

	// 要发送的 API 请求
	url := fmt.Sprintf("http://localhost:%v/api/status", frpAdminPort)
	method := "GET"

	// 认证信息
	username := "admin"
	password := "admin"

	// 创建 HTTP 客户端
	client := &http.Client{}

	// 创建请求对象
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println("创建请求失败:", err)
		return
	}

	// 添加 Basic Authentication 请求头
	auth := username + ":" + password
	base64Auth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Add("Authorization", "Basic "+base64Auth)

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("发送请求失败:", err)
		return
	}
	defer resp.Body.Close()

	// 处理响应

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("读取响应失败:", err)
		return
	}

	// 将响应体解析为 JSON 格式
	var innerProxys message.InnerProxyStatus
	err = json.Unmarshal(body, &innerProxys)
	if err != nil {
		fmt.Println("解析 JSON 失败:", err)
		return
	}

	proxyRunStatus = map[string]message.TCPProxyStatsu{}
	for _, ts := range innerProxys.TCP {
		proxyRunStatus[ts.Name] = ts
	}

	// 打印 JSON 数据
	fmt.Println(innerProxys)

	fmt.Println("请求成功", proxyRunStatus)

}

// tryGetProxyManager 尝试获取代理管理器
func getServiceInfo() message.ServiceInfo {
	if serverIp == "" || serverPort == 0 {
		defer func() {
			if run == 1 {
				server.Close()
			}
		}()
		return message.ServiceInfo{
			ServerIp:   "127.0.0.1",
			ServerPort: 0,
			RunStatus:  0,
			Time:       time.Now().UnixNano(),
		}
	} else {
		return message.ServiceInfo{
			ServerIp:   serverIp,
			ServerPort: serverPort,
			RunStatus:  run,
			Time:       time.Now().UnixNano(),
		}
	}
}
