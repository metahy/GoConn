package main

import (
	"log"
	"net/http"
	"time"
	"encoding/json"

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
	defer func() {
		if c.room != nil {
			//c.room.unregister <- c
			delete(c.room.clients, c)
			close(c.send)
			msg := make(map[string]interface{})
			msg["msgtype"] = 1
			if c.userid != "" {
				msg["msg"] = c.username + "离开直播间"
				log.Println(c.username, "<-", c.room.roomid, "| 当前人数：", len(c.room.clients))
				msg["clientnum"] = len(c.room.clients)
				msgbytes, _ := json.Marshal(msg)
				c.room.broadcast <- msgbytes
			} else {
				//msg["msg"] = "访客离开直播间"
				log.Println("访客 <-", c.room.roomid, "| 当前人数：", len(c.room.clients))
				msg["clientnum"] = len(c.room.clients)
				msgbytes, _ := json.Marshal(msg)
				c.room.broadcast <- msgbytes
			}
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
		//message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		var r map[string]interface{}
		if json.Unmarshal(message, &r) != nil {
			continue
		}
		msgtype := int(r["msgtype"].(float64))
		msg := make(map[string]interface{})
		msg["msgtype"] = msgtype
		var msgbytes []byte
		switch msgtype {
		case 1:	// 进入直播间
			roomid := r["roomid"].(string)
			if _, ok := rooms[roomid]; !ok {
				log.Println(roomid, "房间发言频道不存在 -> 创建")
				// 模拟根据房间号找主播昵称
				rooms[roomid] = newRoom(roomid, "主播" + roomid)
				go rooms[roomid].run()
			}
			c.room = rooms[roomid]
			userid, useridin := "", r["userid"]
			if useridin != nil {
				userid = useridin.(string)
			}
			if userid != "" {
				c.userid = userid
				c.username = r["username"].(string)
				c.room.clients[c] = true
				log.Println(c.username, "->", roomid, "| 当前人数：", len(c.room.clients))
				msg["msg"] = c.username + "进入直播间"
			} else {
				c.room.clients[c] = true
				log.Println("访客 ->", roomid, "| 当前人数：", len(c.room.clients))
				//msg["msg"] = "访客进入直播间"
			}
			msg["clientnum"] = len(c.room.clients)
		case 2:	// 发弹幕
			words := r["msg"].(string)
			msg["username"] = c.username
			msg["msg"] = words
			log.Println(c.username, "|", c.room.roomid, "->", words)
		case 3:	// 刷礼物
			msg["username"] = c.username
			giftlevel := int(r["giftlevel"].(float64))
			msg["giftlevel"] = giftlevel
			log.Println(c.username, "|", c.room.roomid, "-> Gift:", giftlevel)
			if giftlevel == 2 {
				msg["anchor"] = c.room.anchor
				msgbytes, _ := json.Marshal(msg)
				for _, roomeach := range rooms {
					roomeach.broadcast <- msgbytes
				}
				continue
			}
		}
		msgbytes, _ = json.Marshal(msg)
		c.room.broadcast <- msgbytes
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{conn: conn, send: make(chan []byte, 256)}

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}