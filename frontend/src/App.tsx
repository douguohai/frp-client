import React from 'react';

import '@fortawesome/fontawesome-free/css/all.css';
import '@fortawesome/fontawesome-free/css/v4-shims.css';

import 'amis/lib/themes/cxd.css';
import 'amis/lib/helper.css';
import 'amis/sdk/iconfont.css';

import axios from 'axios';
import copy from 'copy-to-clipboard';

import { addRule, AlertComponent, render as renderAmis, ToastComponent } from 'amis';
import { toast } from 'amis-ui';
import Icon from './assert/4873dbfaf6a5.png'
import { SetFrpServiceConfig } from "../wailsjs/go/main/App";

// amis 环境配置
const env = {
    // 下面三个接口必须实现
    fetcher: ({
        url, // 接口地址
        method, // 请求方法 get、post、put、delete
        data, // 请求数据
        responseType,
        config, // 其他配置
        headers // 请求头
    }: any) => {
        config = config || {};
        config.withCredentials = true;
        responseType && (config.responseType = responseType);

        if (config.cancelExecutor) {
            config.cancelToken = new (axios as any).CancelToken(
                config.cancelExecutor
            );
        }

        config.headers = headers || {};

        if (method !== 'post' && method !== 'put' && method !== 'patch') {
            if (data) {
                config.params = data;
            }
            return (axios as any)[method](url, config);
        } else if (data && data instanceof FormData) {
            config.headers = config.headers || {};
            config.headers['Content-Type'] = 'multipart/form-data';
        } else if (
            data &&
            typeof data !== 'string' &&
            !(data instanceof Blob) &&
            !(data instanceof ArrayBuffer)
        ) {
            data = JSON.stringify(data);
            config.headers = config.headers || {};
            config.headers['Content-Type'] = 'application/json';
        }

        return (axios as any)[method](url, data, config);
    },
    isCancel: (value: any) => (axios as any).isCancel(value),
    copy: (content: string) => {
        copy(content);
        toast.success('内容已复制到粘贴板');
    },
    // 后面这些接口可以不用实现

    // 默认是地址跳转
    // jumpTo: (
    //   location: string /*目标地址*/,
    //   action: any /* action对象*/
    // ) => {
    //   // 用来实现页面跳转, actionType:link、url 都会进来。
    // },

    // updateLocation: (
    //   location: string /*目标地址*/,
    //   replace: boolean /*是replace，还是push？*/
    // ) => {
    //   // 地址替换，跟 jumpTo 类似
    // }

    // isCurrentUrl: (
    //   url: string /*url地址*/,
    // ) => {
    //   // 用来判断是否目标地址当前地址
    // },

    // notify: (
    //   type: 'error' | 'success' /**/,
    //   msg: string /*提示内容*/
    // ) => {
    //   toast[type]
    //     ? toast[type](msg, type === 'error' ? '系统错误' : '系统消息')
    //     : console.warn('[Notify]', type, msg);
    // },
    // alert,
    // confirm,
};

addRule(
    // 校验名
    'isIPV4',
    // 校验函数，values 是表单里所有表单项的值，可用于做联合校验；value 是当前表单项的值
    (values, value) => {
        // 使用正则表达式检查IPv4格式
        const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/;
        if (!ipv4Regex.test(value)) {
            return false;
        }

        // 将地址拆分成四个部分
        const parts = value.split('.');
        for (let i = 0; i < parts.length; i++) {
            const part = parseInt(parts[i]);
            // 检查每个部分是否在0到255之间
            if (isNaN(part) || part < 0 || part > 255) {
                return false;
            }
        }
        return true;
    },
    // 出错时的报错信息
    '输入的不是ipv4'
);


class AMISComponent extends React.Component<any, any> {

    constructor(props: any) {
        super(props);
    }


    render() {

        function handleBroadcast(type: string, rawEvent: any, data: any) {
            switch (type) {
                case 'SaveConfig':
                    console.log({ type: type, body: data })
            }
        }


        return renderAmis(
            // 这里是 amis 的 Json 配置。
            {
                "type": "page",
                "body": [
                    {
                        "type": "form",
                        "initApi": {
                            "url": "/api/getServer",
                            "data": {
                                time: "${time}"
                            },
                        },
                        // "interval": 20000,
                        "api": {
                            "url": "/api/connect",
                            "data": {
                                serverIp: "${serverIp}",
                                serverPort: "${serverPort}"
                            },
                        },
                        "id": "server-config-form",
                        "title": "服务器设置",
                        "body": [
                            {
                                "type": "input-text",
                                "name": "serverIp",
                                "id": "server-ip-id",
                                "label": "服务器地址",
                                "validateOnChange": true,
                                "validations": {
                                    "isIPV4": true
                                }
                            },
                            {
                                "type": "input-number",
                                "name": "serverPort",
                                "id": "server-port-id",
                                "label": "端口",
                                "step": 1,
                                "min": 1,
                                "max": 65535
                            },
                            {
                                "type": "button",
                                "icon": "fas fa-globe-asia",
                                "level": "info",
                                "id": "connect-id",
                                "label": "连接",
                                "tooltip": "连接远程服务器并进行验证",
                                "onEvent": {
                                    "click": {
                                        "actions": [
                                            {
                                                "actionType": "disabled",
                                                "componentId": "connect-id"
                                            },
                                            {
                                                "actionType": "submit",
                                                "componentId": "server-config-form"
                                            }
                                        ]
                                    }
                                }
                            },
                            {
                                "type": "button",
                                "icon": "far fa-stop-circle",
                                "level": "danger",
                                "id": "disconnect-server-button",
                                "label": "中断",
                                "tooltip": "断后如当前存在正在运行的映射，将全部中断",
                                "onEvent": {
                                    "click": {
                                        "actions": [
                                            {
                                                "actionType": "ajax",
                                                "args": {
                                                    "api": {
                                                        "url": "/api/unlock",
                                                        "method": "get"
                                                    },
                                                    "data": {
                                                        "serverIp": "${serverIp}",
                                                        "serverPort": "${serverPort}"
                                                    }
                                                },
                                            },
                                            {
                                                "actionType": "toast",
                                                "args": {
                                                    "msg": "${responseMsg}"
                                                }
                                            },
                                            {
                                                "actionType": "parallel",
                                                "expression": "${event.data.responseResult.responseStatus === 0}",
                                                "children": [
                                                    {
                                                        "actionType": "enabled",
                                                        "componentId": "server-ip-id"
                                                    },
                                                    {
                                                        "actionType": "enabled",
                                                        "componentId": "server-port-id"
                                                    },
                                                    {
                                                        "actionType": "disabled",
                                                        "componentId": "disconnect-server-button"
                                                    },
                                                    {
                                                        "actionType": "enabled",
                                                        "componentId": "connect-id"
                                                    },
                                                ]
                                            }
                                        ]
                                    }
                                }
                            }
                        ],
                        "onEvent": {
                            "submitSucc": {
                                "actions": [
                                    {
                                        "actionType": "parallel",
                                        "expression": "${event.data.result.status  === 0}",
                                        "children": [
                                            {
                                                "actionType": "disabled",
                                                "componentId": "server-ip-id"
                                            },
                                            {
                                                "actionType": "disabled",
                                                "componentId": "server-port-id"
                                            },
                                            {
                                                "actionType": "disabled",
                                                "componentId": "connect-id"
                                            },
                                            {
                                                "actionType": "enabled",
                                                "componentId": "disconnect-server-button"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "submitFail": {
                                "actions": [
                                    {
                                        "actionType": "enabled",
                                        "expression": "${event.data.result.status  !== 0}",
                                        "componentId": "connect-id"
                                    },
                                ]
                            },
                            "inited": {
                                "actions": [
                                    {
                                        "actionType": "parallel",
                                        "children": [
                                            {
                                                "actionType": "disabled",
                                                "componentId": "server-ip-id",
                                                "expression": "${event.data.runStatus==1}",
                                            },
                                            {
                                                "actionType": "disabled",
                                                "componentId": "server-port-id",
                                                "expression": "${event.data.runStatus==1}",
                                            },
                                            {
                                                "actionType": "disabled",
                                                "componentId": "connect-id",
                                                "expression": "${event.data.runStatus==1}",
                                            },
                                            {
                                                "actionType": "disabled",
                                                "componentId": "disconnect-server-button",
                                                "expression": "${event.data.runStatus!=1}",
                                            }
                                        ]
                                    }
                                ]
                            }
                        },
                        "mode": "inline"
                    },
                    {
                        "type": "divider",
                    },
                    {
                        "type": "button",
                        "icon": "fas fa-plus-square",
                        "actionType": "dialog",
                        "level": "warning",
                        "dialog": {
                            "title": "新增穿刺",
                            "id": "add-proxy-id",
                            "actions": [
                                {
                                    "label": "新增",
                                    "actionType": "submit",
                                    "primary": true,
                                    "type": "button"
                                }
                            ],
                            "body": {
                                "type": "form",
                                "api": {
                                    "url": "/api/addProxy",
                                    "method": "post",
                                },
                                "closeDialogOnSubmit": true,
                                "reload": "card-service-id",
                                "body": [
                                    {
                                        "type": "input-text",
                                        "name": "proxyName",
                                        "label": "代理名称",
                                        "required": true
                                    },
                                    {
                                        "type": "divider"
                                    },
                                    {
                                        "type": "input-number",
                                        "name": "localPort",
                                        "label": "本地端口",
                                        "required": true,
                                        "step": 1,
                                        "min": 1,
                                        "max": 65535
                                    },
                                    {
                                        "type": "divider"
                                    },
                                    {
                                        "type": "input-number",
                                        "name": "remotePort",
                                        "label": "远程端口",
                                        "required": true,
                                        "step": 1,
                                        "min": 1,
                                        "max": 65535
                                    },
                                    {
                                        "type": "divider"
                                    }
                                ],

                            },
                        },
                        "label": "新增映射",
                    },
                    {
                        "type": "divider"
                    },
                    {
                        "type": "service",
                        "id": "card-service-id",
                        "api": {
                            "url": "/api/getProxy",
                            "method": "get",
                            "replaceData": true,
                            "data": {
                                time: "${time}"
                            },
                        },
                        "interval": 5000,
                        "silentPolling": true,
                        "body": [
                            {
                                "mode": "cards",
                                "source": "$rows",
                                "id": "cards-id",
                                "multiple": false,
                                "type": "crud",
                                "card": {
                                    "toolbar": [
                                        {
                                            "type": "tooltip-wrapper",
                                            "content": "${status}",
                                            "body": {
                                                "type": "switch",
                                                "onText": "已开启",
                                                "offText": "已关闭",
                                                "name": "status",
                                                "onEvent": {
                                                    "change": {
                                                        "actions": [
                                                            {
                                                                "actionType": "ajax",
                                                                "args": {
                                                                    "api": {
                                                                        "url": "/api/openProxy",
                                                                        "method": "put",
                                                                        "data": {
                                                                            "status": "${status}",
                                                                            "proxyName": "${proxyName}"
                                                                        },
                                                                    }
                                                                }
                                                            },
                                                            {
                                                                "actionType": "static",
                                                                "componentName": "status",
                                                            }

                                                        ]
                                                    }
                                                }
                                            }
                                        },

                                    ],
                                    "header": {
                                        "title": "${proxyName}",
                                        "subTitle": "${proxyType}",
                                        "subTitlePlaceholder": "${status}",
                                        "avatar": Icon,
                                        "avatarClassName": "pull-left thumb b-3x m-r"
                                    },
                                    "body": [
                                        {
                                            "label": "本地端口",
                                            "name": "localPort"
                                        },
                                        {
                                            "name": "remotePort",
                                            "label": "远程端口"
                                        },
                                        {
                                            "name": "remoteAddr",
                                            "label": "访问链接",
                                            // "style":{
                                            //     "fontSize:11"
                                            // },
                                            "className": "text-blue-400 m:text-red-400",
                                            "onEvent": {
                                                "click": {
                                                    "actions": [
                                                        {
                                                            "actionType": "copy",
                                                            "args": {
                                                                "content": "${remoteAddr}"
                                                            }
                                                        }
                                                    ]
                                                }
                                            }

                                        },
                                    ],
                                    "actions": [
                                        {
                                            "type": "button",
                                            "icon": "fa fa-pencil",
                                            "actionType": "dialog",
                                            "dialog": {
                                                "title": "编辑",
                                                "body": {
                                                    "type": "form",
                                                    "reload": "card-service-id",
                                                    "api": {
                                                        "url": "/api/editProxy",
                                                        "method": "post",
                                                    },
                                                    "body": [
                                                        {
                                                            "type": "input-text",
                                                            "name": "proxyName",
                                                            "label": "代理名称",
                                                            "required": true,
                                                            "readOnly": true,
                                                        },
                                                        {
                                                            "type": "divider"
                                                        },
                                                        {
                                                            "type": "input-number",
                                                            "name": "localPort",
                                                            "label": "本地端口",
                                                            "required": true,
                                                            "step": 1,
                                                            "min": 1,
                                                            "max": 65535
                                                        },
                                                        {
                                                            "type": "divider"
                                                        },
                                                        {
                                                            "type": "input-number",
                                                            "name": "remotePort",
                                                            "label": "远程端口",
                                                            "required": true,
                                                            "step": 1,
                                                            "min": 1,
                                                            "max": 65535
                                                        },
                                                        {
                                                            "type": "divider"
                                                        }
                                                    ],
                                                    "action": [
                                                        {
                                                            "label": "提交表单",
                                                            "actionType": "submit",
                                                            "primary": true,
                                                            "type": "button"
                                                        }
                                                    ]
                                                }
                                            },
                                            "label": "编辑"
                                        },
                                        {
                                            "type": "button",
                                            "icon": "fa fa-trash",
                                            "actionType": "dialog",
                                            "dialog": {
                                                "title": "提示",
                                                "body": "是否确认删除该配置",
                                                "onEvent": {
                                                    "confirm": {
                                                        "actions": [
                                                            {
                                                                "label": "确认删除",
                                                                "actionType": "ajax",
                                                                "primary": true,
                                                                "type": "button",
                                                                "api": {
                                                                    "url": "/api/delProxy",
                                                                    "method": "post",
                                                                    "data": {
                                                                        "proxyName": "${proxyName}",
                                                                    },
                                                                    "messages": {
                                                                        "success": "成功了！欧耶",
                                                                        "failed": "失败了呢。。"
                                                                    },
                                                                },
                                                            },
                                                            {
                                                                "actionType": "reload",
                                                                "componentId": "card-service-id",
                                                            }
                                                        ],
                                                    }
                                                }
                                            },
                                            "label": "删除"
                                        }
                                    ]
                                }
                            }
                        ],
                    }
                ]
            },
            {
                // props...
                onBroadcast: handleBroadcast
            }
            ,
            env
        );
    }
}


class APP extends React.Component<any, any> {

    render() {

        return (
            <>
                <ToastComponent key="toast" position={'top-right'} />
                <AlertComponent key="alert" />
                <AMISComponent />
            </>
        );
    }
}

export default APP;
