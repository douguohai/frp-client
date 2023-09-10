package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/douguohai/frp-client/message"
	"github.com/douguohai/frp-client/utils"
	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/client/proxy"
	"github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/pkg/consts"
	"github.com/gorilla/mux"
	"github.com/sdomino/scribble"
)

var (
	serverPort, frpAdminPort int

	server *client.Service

	// 服务器IP
	serverIp string = ""

	serverCfg = config.GetDefaultClientConf()

	ctx = context.Background()

	run int64 = 0

	// 创建定时任务，每秒执行一次
	ticker = time.NewTicker(time.Second * 5)

	dir = "./store"

	db *scribble.Driver
)

func init() {
	var err error
	db, err = scribble.New(dir, nil)
	if err != nil {
		fmt.Println("[db init failed]", err)
	} else {
		fmt.Println("[db init successfull]", db)
	}
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
			buildFail(writer, err.Error(), nil)
			return
		}

		if err := addProxy(proxy); err != nil {
			buildFail(writer, err.Error(), nil)
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
func addProxy(proxy message.ProxyMsg) error {
	//判断是否存在重名服务，只检测本地名称
	proxys := getProxyFromDb(proxy.ProxyName)
	if len(proxys) != 0 {
		return errors.New("该服务已经存在,请更换服务名")
	}

	proxy.ProxyName = strings.Trim(proxy.ProxyName, " ")
	proxy.AddTime = time.Now().UnixNano()
	proxy.RemoteProxyName = fmt.Sprintf("%v_%v", proxy.ProxyName, proxy.AddTime)
	proxy.Status = false
	proxy.Type = "tcp"

	_, err := getProxyCfg(proxy)
	if err != nil {
		fmt.Println(err)
		return errors.New("核验配置错误")
	}

	if err := db.Write("proxys", proxy.ProxyName, proxy); err != nil {
		fmt.Println("[db]-[insert] 添加数据库失败 ", err)
		return errors.New("添加数据库失败")
	} else {
		fmt.Println("添加成功")
	}
	return nil
}

// getProxy 获取代理列表
func getProxy() []message.ProxyMsgVo {
	proxys := getProxyFromDb("")

	values := make([]message.ProxyMsgVo, 0)

	for _, value := range proxys {
		proxyType := strings.ToLower(value.Type)
		switch proxyType {
		case "tcp":
			tempStatus := value.Status
			values = append(values, message.ProxyMsgVo{
				ProxyName:  value.ProxyName,
				Type:       value.Type,
				LocalPort:  value.LocalPort,
				RemotePort: value.RemotePort,
				Status:     tempStatus,
				RemoteAddr: value.RemoteAddr,
				AddTime:    value.AddTime,
			})
		}
	}
	//对value 进行排序
	sort.Slice(values, func(i, j int) bool {
		return values[i].AddTime > values[j].AddTime
	})
	return values
}

// addProxy 添加代理
// proxy 代理信息
func editProxy(proxy message.ProxyMsg) error {

	proxys := getProxyFromDb(strings.Trim(proxy.ProxyName, " "))
	if len(proxys) != 1 {
		return errors.New("不存在该名称的代理")
	}
	temp := proxys[0]

	temp.LocalPort = proxy.LocalPort
	temp.RemotePort = proxy.RemotePort
	temp.Status = false

	_, err := getProxyCfg(temp)
	if err != nil {
		fmt.Println(err)
		return errors.New("核验配置错误")
	}

	if err := db.Write("proxys", temp.ProxyName, temp); err != nil {
		log.Print(err)
		return errors.New("修改异常")
	}

	reloadConfigFromDb()
	return nil
}

// addProxy 添加代理
// proxy 代理信息
func delProxy(delProxy message.ProxyMsg) (err error) {
	proxys := getProxyFromDb(strings.Trim(delProxy.ProxyName, " "))
	if len(proxys) != 1 {
		return errors.New("不存在该名称的代理")
	}
	temp := proxys[0]

	// Delete a fish from the database
	if err := db.Delete("proxys", temp.ProxyName); err != nil {
		log.Print("Error", err)
	}

	//判断当前代理如果处于运行中,等待关闭，重新刷新配置
	reloadConfigFromDb()

	return nil
}

// addProxy 添加代理
// proxy 代理信息
func openProxy(proxyStatus message.ProxyStatus) error {
	proxys := getProxyFromDb(strings.Trim(proxyStatus.ProxyName, " "))
	if len(proxys) != 1 {
		return errors.New("不存在该名称的代理")
	}
	temp := proxys[0]

	temp.Status = proxyStatus.Status

	if err := db.Write("proxys", temp.ProxyName, temp); err != nil {
		log.Print(err)
		return errors.New("开启失败")
	}

	reloadConfigFromDb()
	return nil
}

// connectFrpServer 连接frp服务器
func connectFrpServer(ch chan int) {
	if run == 1 {
		closeAllProxy()
		server.Close()
	}
	run = -1
	serverCfg.AdminUser = "admin"
	serverCfg.AdminPwd = "admin"
	serverCfg.DialServerTimeout = 3
	frpAdminPort, _ = utils.GetAvailablePort()
	serverCfg.AdminPort = frpAdminPort

	activityProxyConfList := map[string]config.ProxyConf{}

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
		closeAllProxy()
		reloadConfigFromDb()
		ticker.Stop()
	}
}

// unlockConfig 解锁frp服务器配置
func unlockConfig() {
	serverCfg.ServerAddr = ""
	serverCfg.ServerPort = 0
	closeAllProxy()
	reloadConfigFromDb()
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
		}
	}
}

// tryGetProxyManager 尝试获取代理管理器
func getProxyStatus() {

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

	proxyRunStatus := map[string]message.TCPProxyStatsu{}

	for _, ts := range innerProxys.TCP {
		proxyRunStatus[ts.Name] = ts
	}

	localProxy := getProxyFromDb("")

	for _, localTemp := range localProxy {
		temp, has := proxyRunStatus[localTemp.RemoteProxyName]
		if !has {
			localTemp.Status = false
			localTemp.RemoteAddr = "暂无"
		} else {
			localTemp.RunStatus = temp.Status
			localTemp.RemoteAddr = temp.RemoteAddr
		}
		localTemp.RunStatus = temp.Status
		if err := db.Write("proxys", localTemp.ProxyName, localTemp); err != nil {
			log.Print(err.Error())
		}
	}

	log.Println("请求成功", proxyRunStatus)

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

// getProxyFromDb 数据库获取代理信息
// filter 过滤字段，代理名称
func getProxyFromDb(filter string) []message.ProxyMsg {
	proxys := []message.ProxyMsg{}
	records, err := db.ReadAll("proxys")
	if err != nil {
		fmt.Println("Error", err)
		return proxys
	}
	for _, f := range records {
		temp := message.ProxyMsg{}
		if err := json.Unmarshal([]byte(f), &temp); err != nil {
			fmt.Println("Error", err)
		}
		if filter == "" {
			proxys = append(proxys, temp)
		}
		if filter != "" && temp.ProxyName == filter {
			proxys = append(proxys, temp)
		}
	}
	return proxys
}

func closeAllProxy() error {
	records, err := db.ReadAll("proxys")
	if err != nil {
		fmt.Println("Error", err)
		return errors.New("关闭失败")
	}
	for _, f := range records {
		temp := message.ProxyMsg{}
		if err := json.Unmarshal([]byte(f), &temp); err != nil {
			fmt.Println("Error", err)
		}
		temp.Status = false
		temp.RemoteAddr = "暂无"
		temp.RunStatus = proxy.ProxyPhaseClosed
		if err := db.Write("proxys", temp.ProxyName, temp); err != nil {
			log.Print(err.Error())
		}
	}
	return err
}

// reloadConfigFromDb 数据库重新刷新配置
func reloadConfigFromDb() {
	proxys := getProxyFromDb("")
	if len(proxys) == 0 {
		return
	}

	proxyConfList := map[string]config.ProxyConf{}
	//数据库读取所有配置
	for _, temp := range proxys {
		//如果预期状态为打开，进行配置文件转换
		if temp.Status {
			cfg, err := getProxyCfg(temp)
			if err != nil {
				fmt.Println("Error", err)
				continue
			}
			proxyConfList[temp.RemoteProxyName] = cfg
		}
	}
	//reload 配置
	if run == 1 {
		server.ReloadConf(proxyConfList, nil)
	}

}

// addProxy 添加代理
// proxy 代理信息
func getProxyCfg(proxy message.ProxyMsg) (*config.TCPProxyConf, error) {
	cfg := &config.TCPProxyConf{}
	var err error
	cfg.ProxyName = proxy.RemoteProxyName

	cfg.ProxyType = consts.TCPProxy
	cfg.LocalIP = "127.0.0.1"
	cfg.LocalPort = proxy.LocalPort
	cfg.RemotePort = proxy.RemotePort
	cfg.UseEncryption = false
	cfg.UseCompression = false
	cfg.BandwidthLimit, _ = config.NewBandwidthQuantity("")
	cfg.BandwidthLimitMode = config.BandwidthLimitModeClient

	err = cfg.ValidateForClient()
	if err != nil {
		fmt.Println("[init cfg error]")
	}
	return cfg, err
}
