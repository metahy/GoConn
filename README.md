# 基于Go的WebSocket直播间推送系统

### 用户行为分析
#### 进入直播间：
- 1、访客进入
- 2、已登录用户进入

#### 离开直播间：
- 1、访客离开
- 2、已登录用户用户离开

#### 发弹幕：

#### 送礼物：
- 1、普通礼物
- 2、高级礼物

### 推送范围及内容分析

<table>
 <tr>
    <th>用户行为</th>
    <th>类别</th>
    <th>推送范围</th>
    <th>推送描述</th>
 </tr>
 <tr>
    <td rowspan="2">进入直播间</td>
    <td>访客</td>
    <td>直播间内推送</td>
    <td>推送观看人数变化</td>
 </tr>
 <tr>
    <td>已登录用户</td>
    <td>直播间内推送</td>
    <td>推送用户信息及人数变化</td>
 </tr>
 <tr>
    <td rowspan="2">离开直播间</td>
    <td>访客</td>
    <td>直播间内推送</td>
    <td>推送观看人数变化</td>
 </tr>
 <tr>
    <td>已登录用户</td>
    <td>直播间内推送</td>
    <td>推送用户信息及人数变化</td>
 </tr>
 <tr>
    <td>发送弹幕</td>
    <td></td>
    <td>直播间内推送</td>
    <td>推送用户信息及弹幕</td>
 </tr>
 <tr>
    <td rowspan="2">送礼物</td>
    <td>普通礼物</td>
    <td>直播间内推送</td>
    <td>推送用户信息及礼物</td>
 </tr>
 <tr>
    <td>高级礼物</td>
    <td>全部直播间推送</td>
    <td>推送主播信息、用户信息及礼物</td>
 </tr>
</table>

### 请求格式规范
#### 进入直播间：
访客：
```
{
    “msgtype”: 1
    “roomid”: “12345”
}
```
已登录用户：
```
{
    “msgtype”: 1
    “roomid”: “12345”
    “userid: “zhangsan”
    “username”: “张三”
}
```

#### 离开直播间：
断开连接

#### 发弹幕：
```
{
    “msgtype”: 2
    “msg”: “测试哈哈哈”
}
```

#### 刷礼物：
```
{
    “msgtype”: 3
    “giftlevel”: 1		// 普通礼物当前直播间内推送
}

{
    “msgtype”: 3
    “giftlevel”: 2		// 高级礼物全部直播间内推送
}
```

### 推送格式规范

#### 进入、离开直播间推送人数变化及用户信息：
```
已登录用户
{
    “msgtype”: 1
    “clientnum”: 123
    “msg”: “张三进入直播间”
}
{
    “msgtype”: 1
    “clientnum”: 123
    “msg”: “张三离开直播间”
}
访客
{
    “msgtype”: 1
    “clientnum”: 123
}
```

#### 发弹幕：
```
{
    “msgtype”: 2
    “username”: “张三”
    “msg”: “666”
}
```

#### 刷礼物：
```
普通
{
    “msgtype”: 3
    “username”: “张三”
    “giftlevel”: 1
}
高级
{
    “msgtype”: 3
    “username”: “张三”
    “anchor”: “dada”
    “giftlevel”: 1
}
```

>其他说明：

为了模拟不同用户在不同直播间内的行为，通过四个html页面进行演示
- home.html 模拟用户张三访问12345直播间  
- home2.html 模拟未登录访客访问12345直播间  
- home3.html 模拟用户李四访问12346直播间  
- home4.html 模拟未登录访客访问12346直播间