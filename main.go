package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	_ "modernc.org/sqlite"
)

const AdminSecret = "admin666"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type ClientRequest struct {
	Action   string `json:"action"`
	Username string `json:"username"`
	Password string `json:"password"`
	Content  string `json:"content"`
}

type ServerResponse struct {
	Type    string `json:"type"`
	Sender  string `json:"sender"`
	Content string `json:"content"`
}

type Hub struct {
	clients    map[*websocket.Conn]string
	broadcast  chan ServerResponse
	mutex      sync.Mutex
	db         *sql.DB
	globalMute bool
}

func getClientIP(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		if ip := r.Header.Get(header); ip != "" {
			addresses := strings.Split(ip, ",")
			trimmedIP := strings.TrimSpace(addresses[0])
			if trimmedIP != "" && !strings.HasPrefix(trimmedIP, "169.254") {
				return trimmedIP
			}
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if strings.HasPrefix(ip, "169.254") {
		return "局域网未知(169过滤)"
	}
	if ip == "::1" || ip == "127.0.0.1" {
		return "服务器本机"
	}
	return ip
}

func main() {
	db, err := sql.Open("sqlite", "./chat.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Exec(`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT UNIQUE, password TEXT, last_ip TEXT DEFAULT '未知');`)
	db.Exec(`CREATE TABLE IF NOT EXISTS messages (id INTEGER PRIMARY KEY AUTOINCREMENT, sender TEXT, content TEXT);`)
	db.Exec(`ALTER TABLE users ADD COLUMN last_ip TEXT DEFAULT '未知';`)

	hub := &Hub{
		clients:   make(map[*websocket.Conn]string),
		broadcast: make(chan ServerResponse),
		db:        db,
	}
	go hub.run()

	http.Handle("/", http.FileServer(http.Dir("./frontend")))
	http.HandleFunc("/ws", hub.handleWebsocket)

	// ==========================================
	// 🛠️ 后台控制中心 API 矩阵
	// ==========================================

	http.HandleFunc("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("secret") != AdminSecret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		onlineMap := make(map[string]bool)
		hub.mutex.Lock()
		for _, name := range hub.clients {
			onlineMap[name] = true
		}
		hub.mutex.Unlock()

		rows, err := db.Query("SELECT id, username, last_ip FROM users ORDER BY id DESC")
		if err != nil { return }
		defer rows.Close()

		type UserInfo struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			LastIP   string `json:"last_ip"`
			IsOnline bool   `json:"is_online"`
		}
		var list []UserInfo
		for rows.Next() {
			var u UserInfo
			rows.Scan(&u.ID, &u.Username, &u.LastIP)
			u.IsOnline = onlineMap[u.Username]
			list = append(list, u)
		}
		json.NewEncoder(w).Encode(list)
	})

	http.HandleFunc("/api/admin/delete-user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "POST" { return }
		var req struct { Secret string `json:"secret"`; Username string `json:"username"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Secret != AdminSecret { w.WriteHeader(http.StatusUnauthorized); return }

		db.Exec("DELETE FROM users WHERE username = ?", req.Username)
		db.Exec("DELETE FROM messages WHERE sender = ?", req.Username)

		hub.mutex.Lock()
		for conn, name := range hub.clients {
			if name == req.Username {
				conn.WriteJSON(ServerResponse{Type: "auth_err", Content: "❌ 你的账号已被管理员强制封禁并销户！"})
				conn.Close()
				delete(hub.clients, conn)
				break
			}
		}
		hub.mutex.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	http.HandleFunc("/api/admin/toggle-mute", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("secret") != AdminSecret { w.WriteHeader(http.StatusUnauthorized); return }

		hub.mutex.Lock()
		hub.globalMute = !hub.globalMute
		isMuted := hub.globalMute
		hub.mutex.Unlock()

		if isMuted {
			hub.broadcast <- ServerResponse{Type: "msg", Sender: "📢 系统通知", Content: "管理员已开启全场禁言！"}
		} else {
			hub.broadcast <- ServerResponse{Type: "msg", Sender: "📢 系统通知", Content: "全场禁言已解除。"}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"global_mute": isMuted})
	})

	http.HandleFunc("/api/admin/broadcast", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "POST" { return }
		var req struct { Secret string `json:"secret"`; Content string `json:"content"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Secret != AdminSecret { w.WriteHeader(http.StatusUnauthorized); return }

		if req.Content != "" {
			hub.broadcast <- ServerResponse{Type: "msg", Sender: "📢 系统公告", Content: req.Content}
			db.Exec("INSERT INTO messages (sender, content) VALUES (?, ?)", "📢 系统公告", req.Content)
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	http.HandleFunc("/api/admin/messages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("secret") != AdminSecret { w.WriteHeader(http.StatusUnauthorized); return }

		rows, _ := db.Query("SELECT id, sender, content FROM messages ORDER BY id DESC LIMIT 100")
		defer rows.Close()

		type MsgInfo struct { ID int `json:"id"`; Sender string `json:"sender"`; Content string `json:"content"` }
		var msgs []MsgInfo
		for rows.Next() {
			var m MsgInfo
			rows.Scan(&m.ID, &m.Sender, &m.Content)
			msgs = append(msgs, m)
		}
		json.NewEncoder(w).Encode(msgs)
	})

	http.HandleFunc("/api/admin/delete-message", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "POST" { return }
		var req struct { Secret string `json:"secret"`; ID int `json:"id"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Secret != AdminSecret { w.WriteHeader(http.StatusUnauthorized); return }

		db.Exec("DELETE FROM messages WHERE id = ?", req.ID)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	http.HandleFunc("/api/admin/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("secret") != AdminSecret { w.WriteHeader(http.StatusUnauthorized); return }
		hub.mutex.Lock()
		defer hub.mutex.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"global_mute": hub.globalMute})
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		fmt.Println("====================================================")
		fmt.Println("👉 聊天大厅: http://localhost:8080")
		fmt.Println("👉 独立登录控制台: http://localhost:8080/admin.html")
		fmt.Println("====================================================")
		openBrowser("http://localhost:8080/admin.html")
	}()

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (h *Hub) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }

	clientIP := getClientIP(r)

	defer func() {
		h.mutex.Lock()
		delete(h.clients, conn)
		h.mutex.Unlock()
		conn.Close()
	}()

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil { break }

		var req ClientRequest
		if err := json.Unmarshal(msgBytes, &req); err != nil { continue }

		switch req.Action {
		case "register":
			_, err := h.db.Exec("INSERT INTO users (username, password, last_ip) VALUES (?, ?, ?)", req.Username, req.Password, clientIP)
			if err != nil {
				conn.WriteJSON(ServerResponse{Type: "auth_err", Content: "❌ 该用户名已被注册！"})
			} else {
				h.mutex.Lock()
				h.clients[conn] = req.Username
				h.mutex.Unlock()
				conn.WriteJSON(ServerResponse{Type: "auth_ok", Content: "成功", Sender: req.Username})

				rows, _ := h.db.Query("SELECT sender, content FROM messages ORDER BY id DESC LIMIT 40")
				var msgs []ServerResponse
				for rows.Next() {
					var m ServerResponse; m.Type = "msg"; rows.Scan(&m.Sender, &m.Content)
					msgs = append(msgs, m)
				}
				rows.Close()
				for i := len(msgs) - 1; i >= 0; i-- { conn.WriteJSON(msgs[i]) }
			}

		case "login":
			var savedPwd string
			err := h.db.QueryRow("SELECT password FROM users WHERE username = ?", req.Username).Scan(&savedPwd)
			if err != nil || savedPwd != req.Password {
				conn.WriteJSON(ServerResponse{Type: "auth_err", Content: "❌ 账号或密码输入有误！"})
			} else {
				h.db.Exec("UPDATE users SET last_ip = ? WHERE username = ?", clientIP, req.Username)

				h.mutex.Lock()
				h.clients[conn] = req.Username
				h.mutex.Unlock()
				conn.WriteJSON(ServerResponse{Type: "auth_ok", Sender: req.Username})

				rows, _ := h.db.Query("SELECT sender, content FROM messages ORDER BY id DESC LIMIT 40")
				var msgs []ServerResponse
				for rows.Next() {
					var m ServerResponse; m.Type = "msg"; rows.Scan(&m.Sender, &m.Content)
					msgs = append(msgs, m)
				}
				rows.Close()
				for i := len(msgs) - 1; i >= 0; i-- { conn.WriteJSON(msgs[i]) }
			}

		case "msg":
			h.mutex.Lock()
			user := h.clients[conn]
			isMuted := h.globalMute
			h.mutex.Unlock()

			if user != "" {
				if isMuted {
					conn.WriteJSON(ServerResponse{Type: "auth_err", Content: "当前处于禁言状态！"})
					continue
				}
				h.db.Exec("INSERT INTO messages (sender, content) VALUES (?, ?)", user, req.Content)
				h.broadcast <- ServerResponse{Type: "msg", Sender: user, Content: req.Content}
			}
		}
	}
}

func (h *Hub) run() {
	for res := range h.broadcast {
		h.mutex.Lock()
		for client := range h.clients { client.WriteJSON(res) }
		h.mutex.Unlock()
	}
}

func openBrowser(url string) {
	if runtime.GOOS == "windows" { exec.Command("roundll32", "url.dll,FileProtocolHandler", url).Start() }
}