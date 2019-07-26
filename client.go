package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
// Client用于封装一个websocket.Conn，代表一个连接，即一个观众
type Client struct {
	// 所在直播间
	room *Room

	// 用户ID
	userid string

	// 用户昵称
	username string

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	// socket连接断开时执行defer，在此处对用户离开直播间行为进行处理
	defer func() {
		if c.room != nil {
			delete(c.room.clients, c) // 移除直播间的当前连接
			close(c.send)             // 关闭当前连接的发送chan
			msg := make(map[string]interface{})
			msg["msgtype"] = 1
			if c.userid != "" { // 已登录用户提示用户离开直播间
				msg["msg"] = c.username + "离开直播间"
				log.Println(c.username, "<-", c.room.roomid, "| 当前人数：", len(c.room.clients))
				msg["clientnum"] = len(c.room.clients)
			} else { // 访客断开连接只推送人数
				log.Println("访客 <-", c.room.roomid, "| 当前人数：", len(c.room.clients))
				msg["clientnum"] = len(c.room.clients)
			}
			msgbytes, _ := json.Marshal(msg)
			c.room.broadcast <- msgbytes
		}
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		log.Println(string(message))
		// 解析json文本
		var r map[string]interface{}
		if json.Unmarshal(message, &r) != nil {
			continue
		}
		msgtype := int(r["msgtype"].(float64))
		msg := make(map[string]interface{})
		msg["msgtype"] = msgtype
		var msgbytes []byte
		// 根据不同的消息类型分别处理
		switch msgtype {
		case 1: // 进入直播间
			roomid := r["roomid"].(string)
			if _, ok := rooms[roomid]; !ok {
				log.Println(roomid, "房间发言频道不存在 -> 创建")
				// 模拟根据房间号找主播昵称
				anchor := "主播" + roomid
				rooms[roomid] = newRoom(roomid, anchor)
				go rooms[roomid].run()
			}
			c.room = rooms[roomid]
			userid, useridin := "", r["userid"]
			if useridin != nil {
				userid = useridin.(string)
			}
			if userid != "" { // userid不为空表示为已登录用户
				c.userid = userid
				c.username = r["username"].(string)
				c.room.clients[c] = true
				log.Println(c.username, "->", roomid, "| 当前人数：", len(c.room.clients))
				msg["msg"] = c.username + "进入直播间"
			} else { // 访客
				c.room.clients[c] = true
				log.Println("访客 ->", roomid, "| 当前人数：", len(c.room.clients))
			}
			msg["clientnum"] = len(c.room.clients)
		case 2: // 发弹幕
			words := r["msg"].(string)
			msg["username"] = c.username
			msg["msg"] = words
			log.Println(c.username, "|", c.room.roomid, "->", words)
		case 3: // 刷礼物
			msg["username"] = c.username
			giftlevel := int(r["giftlevel"].(float64))
			msg["giftlevel"] = giftlevel
			log.Println(c.username, "|", c.room.roomid, "-> Gift:", giftlevel)
			if giftlevel == 2 { // 礼物等级为高级时需要在全部直播间推送，推送时要带着主播信息
				msg["anchor"] = c.room.anchor
				msgbytes, _ := json.Marshal(msg) // 先构造好json，再在循环中使用
				go pushAllRoom(msgbytes)         // 推送至全部直播间
				continue                         // 全部推送完成后跳过本次循环避免后面的房间内推送
			}
		}
		msgbytes, _ = json.Marshal(msg)
		c.room.broadcast <- msgbytes
	}
}

func pushAllRoom(msgbytes []byte) {
	for _, roomeach := range rooms {
		roomeach.broadcast <- msgbytes
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	// 允许跨域
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	// 将请求升级为websocket连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// 将连接包装为一个client，一个client代表一个观众
	client := &Client{conn: conn, send: make(chan []byte, 256)}

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
