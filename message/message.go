package message

const (
	SaveConfig   string = "SaveConfig"   // 跳转登录页面
	MsgParseErr  string = "fail"         // 消息解析异常
	Success      string = "success"      // 消息处理完成
	ConnectError string = "ConnectError" // 消息处理完成
)

type Msg struct {
	Type string      `json:"type"`
	Body interface{} `json:"body"`
}

// ConnectServerMsg 连接服务器消息
type ConnectServerMsg struct {
	ServerIp   string `json:"serverIp"`   // 服务器IP
	ServerPort int    `json:"serverPort"` // 服务器端口
}

// ProxyMsg 新增代理消息
type ProxyMsg struct {
	ProxyName  string `json:"proxyName"`
	LocalPort  int    `json:"localPort"`
	RemotePort int    `json:"remotePort"`
}

// ProxyMsgVo 代理展示消息
type ProxyMsgVo struct {
	ProxyName  string `json:"proxyName"`
	Type       string `json:"type"`
	LocalPort  int    `json:"localPort"`
	RemotePort int    `json:"remotePort"`
	Status     bool   `json:"status"`
}

type ProxyMsgVos struct {
	Items []ProxyMsgVo `json:"rows"`
}

type Result struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

type ProxyResult struct {
	Result
	Data ProxyMsgVos `json:"data"`
}

type AjaxResult struct {
	ResponseStatus int    `json:"responseStatus"`
	ResponseMsg    string `json:"responseMsg"`
}

// ProxyStatus 代理状态
type ProxyStatus struct {
	ProxyName string `json:"proxyName"`
	Status    bool   `json:"status"`
}
