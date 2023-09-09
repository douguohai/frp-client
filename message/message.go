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
	ProxyName       string `json:"proxyName"`       //本地代理名称
	RemoteProxyName string `json:"remoteProxyName"` //远程代理名称
	LocalPort       int    `json:"localPort"`       //本地端口
	RemotePort      int    `json:"remotePort"`      //远程端口
	Type            string `json:"type"`            //代理类型
	Status          bool   `json:"status"`          //代理预期运行状态
	RunStatus       string `json:"runStatus"`       //代理实际运行状态
	AddTime         int64  `json:"addTime"`         //新增时间，排序用
	RemoteAddr      string `json:"remote_addr"`     //远程访问地址
}

// ProxyMsgVo 代理展示消息
type ProxyMsgVo struct {
	ProxyName  string `json:"proxyName"`
	Type       string `json:"type"`
	LocalPort  int    `json:"localPort"`
	RemotePort int    `json:"remotePort"`
	Status     bool   `json:"status"`
	RemoteAddr string `json:"remoteAddr"`
	AddTime    int64  `json:"addTime"` //新增时间，排序用
}

type ProxyMsgVos struct {
	Items []ProxyMsgVo `json:"rows"`
	Time  int64        `json:"time"`
}

type Result struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

type ResultC struct {
	Result
	Data interface{} `json:"data"`
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

// InnerProxyStatus 内部代理状态
type InnerProxyStatus struct {
	TCP []TCPProxyStatsu `json:"tcp"`
}

// TCPProxyStatus
type TCPProxyStatsu struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Err        string `json:"err"`
	LocalAddr  string `json:"local_addr"`
	Plugin     string `json:"plugin"`
	RemoteAddr string `json:"remote_addr"`
}

type ServiceInfo struct {
	ServerIp   string `json:"serverIp"`
	ServerPort int    `json:"serverPort"`
	RunStatus  int64  `json:"runStatus"` //0 未链接 1 已连接 -1 尝试连接中
	Time       int64  `json:"time"`
}

type ServiceResult struct {
	Result
	Data ServiceInfo `json:"data"`
}
