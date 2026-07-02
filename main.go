package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	_ "modernc.org/sqlite"
)

type User struct {
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
	IsOnline  bool   `json:"is_online"`
	LastIP    string `json:"last_ip"`
}

type Message struct {
	ID         int64    `json:"id"`
	TargetType string   `json:"target_type"`
	TargetID   string   `json:"target_id"`
	Sender     string   `json:"sender"`
	Content    string   `json:"content"`
	Timestamp  int64    `json:"timestamp"`
	AvatarURL  string   `json:"avatar_url"`
	ReplyToID  *int64   `json:"reply_to_id,omitempty"`
	ReplyToMsg *Message `json:"reply_to_msg,omitempty"`
}

type AdminMessage struct {
	ID      int64  `json:"ID"`
	Sender  string `json:"sender"`
	Content string `json:"content"`
}

var (
	db          *sql.DB
	clients     = make(map[string]*websocket.Conn)
	globalMute  = false
	stateMutex  sync.RWMutex
	adminSecret = "admin666" // 管理员密码

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func main() {
	if err := os.MkdirAll("./uploads", 0755); err != nil {
		log.Fatalf("无法创建上传目录: %v", err)
	}

	initDB()
	defer db.Close()

	http.Handle("/", http.FileServer(http.Dir("./")))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/api/user/info", handleUserInfo)
	http.HandleFunc("/api/user/update", handleUserUpdate)
	http.HandleFunc("/api/user/avatar", handleUserAvatar)
	http.HandleFunc("/api/user/background", handleUserBackground)
	http.HandleFunc("/api/reset-password", handleResetPassword)

	http.HandleFunc("/api/messages", handleGetMessages)
	http.HandleFunc("/api/group/", handleGroupMembers)
	http.HandleFunc("/api/online-users", handleGetOnlineUsers)

	http.HandleFunc("/api/admin/users", handleAdminUsers)
	http.HandleFunc("/api/admin/messages", handleAdminMessages)
	http.HandleFunc("/api/admin/delete-user", handleAdminDeleteUser)
	http.HandleFunc("/api/admin/delete-message", handleAdminDeleteMessage)
	http.HandleFunc("/api/admin/status", handleAdminStatus)
	http.HandleFunc("/api/admin/toggle-mute", handleAdminToggleMute)
	http.HandleFunc("/api/admin/broadcast", handleAdminBroadcast)
	http.HandleFunc("/api/extensions", handleGetExtensions)
	http.HandleFunc("/api/admin/extensions", handleAdminSetExtensions)
	http.HandleFunc("/api/ai/chat", handleAIChat)
	http.HandleFunc("/api/admin/ai/config", handleAdminSetAIConfig)
	http.HandleFunc("/api/ai/config", handleGetAIConfig)

	port := 40001
	addr := fmt.Sprintf(":%d", port)

	ips := getAllIPs()
	var displays []string
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		tag := ""
		if parsed != nil && isPublicIP(parsed) {
			tag = "(公网)"
		}
		displays = append(displays, fmt.Sprintf("%s:%d%s", ip, port, tag))
	}
	fmt.Printf("局域网聊天室已就绪，启动于\n")
	for _, d := range displays {
		fmt.Printf("  %s\n", d)
	}

	go showStartupWindow(displays)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

func getLocalIP() (string, bool) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1", false
	}

	privateIP := ""
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := extractIP(addr)
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			if isPublicIP(ip) {
				return ip.String(), true
			}
			if privateIP == "" {
				privateIP = ip.String()
			}
		}
	}
	if privateIP != "" {
		return privateIP, false
	}
	return "127.0.0.1", false
}

func getAllIPs() []string {
	var res []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{"127.0.0.1"}
	}
	seen := map[string]bool{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip := extractIP(a)
			if ip == nil || ip.To4() == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				if ip4[0] == 169 && ip4[1] == 254 {
					continue
				}
			}
			s := ip.String()
			if !seen[s] {
				seen[s] = true
				res = append(res, s)
			}
		}
	}
	if len(res) == 0 {
		return []string{"127.0.0.1"}
	}
	return res
}

func showStartupWindow(addrs []string) {
	osType := runtime.GOOS
	switch osType {
	case "windows":
		var b strings.Builder
		b.WriteString("Add-Type -AssemblyName System.Windows.Forms,System.Drawing\n")
		b.WriteString("$form = New-Object System.Windows.Forms.Form\n")
		b.WriteString("$form.Text = '局域网聊天室已就绪'\n")
		b.WriteString("$form.Size = New-Object System.Drawing.Size(600,300)\n")
		b.WriteString("$form.StartPosition = 'CenterScreen'\n")
		b.WriteString("$rtb = New-Object System.Windows.Forms.RichTextBox\n")
		b.WriteString("$rtb.ReadOnly = $true\n")
		b.WriteString("$rtb.BackColor = [System.Drawing.Color]::FromArgb(17,17,17)\n")
		b.WriteString("$rtb.ForeColor = [System.Drawing.Color]::White\n")
		b.WriteString("$rtb.Dock = 'Fill'\n")
		b.WriteString("$rtb.Font = New-Object System.Drawing.Font('Microsoft YaHei',12)\n")
		b.WriteString("$rtb.AppendText('局域网聊天室已就绪，启动于')\n")
		b.WriteString("$rtb.AppendText([char]13 + [char]10)\n")
		for _, a := range addrs {
			esc := strings.ReplaceAll(a, "'", "''")
			b.WriteString(fmt.Sprintf("$addr = '%s'\n", esc))
			b.WriteString("foreach ($ch in $addr.ToCharArray()) {\n")
			b.WriteString("  $c = [System.Drawing.Color]::FromArgb((Get-Random -Minimum 0 -Maximum 256),(Get-Random -Minimum 0 -Maximum 256),(Get-Random -Minimum 0 -Maximum 256))\n")
			b.WriteString("  $rtb.SelectionColor = $c\n")
			b.WriteString("  $rtb.AppendText($ch)\n")
			b.WriteString("}\n")
			b.WriteString("$rtb.AppendText([char]13 + [char]10)\n")
		}
		b.WriteString("$form.Controls.Add($rtb)\n")
		b.WriteString("[void]$form.ShowDialog()\n")

		script := b.String()
		tmp := filepath.Join(os.TempDir(), "chatbox_startup.ps1")
		bomPrefixed := append([]byte{0xEF, 0xBB, 0xBF}, []byte(script)...)
		_ = os.WriteFile(tmp, bomPrefixed, 0644)

		cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmp)
		_ = cmd.Start()

	case "linux":
		if p, _ := exec.LookPath("yad"); p != "" {
			var sb strings.Builder
			sb.WriteString("<b>局域网聊天室已就绪，启动于</b>\\n")
			for _, a := range addrs {
				for _, ch := range a {
					color := fmt.Sprintf("#%06x", randColor())
					esc := ch
					if esc == '&' {
						sb.WriteString("&amp;")
					} else if esc == '<' {
						sb.WriteString("&lt;")
					} else if esc == '>' {
						sb.WriteString("&gt;")
					} else {
						sb.WriteString(fmt.Sprintf("<span foreground='%s'>%s</span>", color, string(esc)))
					}
				}
				sb.WriteString("<br/>")
			}
			text := sb.String()
			// 调用 yad
			cmd := exec.Command("yad", "--title=局域网聊天室已就绪", "--text", text, "--width=600", "--height=300", "--center", "--no-buttons", "--undecorated=false")
			_ = cmd.Start()
			return
		}
		if p, _ := exec.LookPath("zenity"); p != "" {
			var sb strings.Builder
			sb.WriteString("局域网聊天室已就绪，启动于\\n")
			for _, a := range addrs {
				sb.WriteString(a)
				sb.WriteString("\\n")
			}
			cmd := exec.Command("zenity", "--info", "--text", sb.String())
			_ = cmd.Start()
			return
		}
		var htmlb strings.Builder
		htmlb.WriteString("<!doctype html><html><meta charset='utf-8'><body style='background:#111;color:#fff;font-family:sans-serif;padding:20px'>")
		htmlb.WriteString("<h3>局域网聊天室已就绪，启动于</h3><div style='background:#222;padding:12px;border-radius:8px;white-space:pre-wrap'>")
		for _, a := range addrs {
			for _, ch := range a {
				color := fmt.Sprintf("#%06x", randColor())
				htmlb.WriteString(fmt.Sprintf("<span style='color:%s'>%s</span>", color, htmlEscape(string(ch))))
			}
			htmlb.WriteString("<br/>")
		}
		htmlb.WriteString("</div></body></html>")
		tmp := filepath.Join(os.TempDir(), "chatbox_startup.html")
		_ = os.WriteFile(tmp, []byte(htmlb.String()), 0644)
		cmd := exec.Command("xdg-open", tmp)
		_ = cmd.Start()

	case "darwin":
		var htmlb strings.Builder
		htmlb.WriteString("<!doctype html><html><meta charset='utf-8'><body style='background:#111;color:#fff;font-family:sans-serif;padding:20px'>")
		htmlb.WriteString("<h3>局域网聊天室已就绪，启动于</h3><div style='background:#222;padding:12px;border-radius:8px;white-space:pre-wrap'>")
		for _, a := range addrs {
			for _, ch := range a {
				color := fmt.Sprintf("#%06x", randColor())
				htmlb.WriteString(fmt.Sprintf("<span style='color:%s'>%s</span>", color, htmlEscape(string(ch))))
			}
			htmlb.WriteString("<br/>")
		}
		htmlb.WriteString("</div></body></html>")
		tmp := filepath.Join(os.TempDir(), "chatbox_startup.html")
		_ = os.WriteFile(tmp, []byte(htmlb.String()), 0644)
		cmd := exec.Command("open", tmp)
		_ = cmd.Start()

	default:
		var htmlb strings.Builder
		htmlb.WriteString("<!doctype html><html><meta charset='utf-8'><body style='background:#111;color:#fff;font-family:sans-serif;padding:20px'>")
		htmlb.WriteString("<h3>局域网聊天室已就绪，启动于</h3><div style='background:#222;padding:12px;border-radius:8px;white-space:pre-wrap'>")
		for _, a := range addrs {
			for _, ch := range a {
				color := fmt.Sprintf("#%06x", randColor())
				htmlb.WriteString(fmt.Sprintf("<span style='color:%s'>%s</span>", color, htmlEscape(string(ch))))
			}
			htmlb.WriteString("<br/>")
		}
		htmlb.WriteString("</div></body></html>")
		tmp := filepath.Join(os.TempDir(), "chatbox_startup.html")
		_ = os.WriteFile(tmp, []byte(htmlb.String()), 0644)
		cmd := exec.Command("xdg-open", tmp)
		_ = cmd.Start()
	}
}

func randColor() int {
	return int(time.Now().UnixNano() & 0xFFFFFF)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func extractIP(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		return !isPrivateIP(ip4)
	}
	return false
}

func isPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}
	return false
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./chat.db")
	if err != nil {
		log.Fatalf("无法打开数据库文件: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password TEXT NOT NULL,
		avatar_url TEXT DEFAULT '',
		signature TEXT DEFAULT '',
		background_url TEXT DEFAULT '',
		last_ip TEXT DEFAULT ''
		);`)
	if err != nil {
		log.Fatalf("创建users表失败: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS session_tokens (
		token TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		created_at INTEGER DEFAULT (strftime('%s','now'))
	);`)
	if err != nil {
		log.Fatalf("创建session_tokens表失败: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS friends (
		username TEXT,
		friend_username TEXT,
		PRIMARY KEY (username, friend_username)
	);`)
	if err != nil {
		log.Fatalf("创建friends表失败: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		owner TEXT NOT NULL
	);`)
	if err != nil {
		log.Fatalf("创建groups表失败: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS group_members (
		group_id INTEGER,
		username TEXT,
		PRIMARY KEY (group_id, username)
	);`)
	if err != nil {
		log.Fatalf("创建group_members表失败: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_type TEXT,
		target_id TEXT,
		sender TEXT,
		content TEXT,
		timestamp INTEGER,
		avatar_url TEXT,
		reply_to_id INTEGER
	);`)
	if err != nil {
		log.Fatalf("创建messages表失败: %v", err)
	}

	_, _ = db.Exec("INSERT OR IGNORE INTO users (username, password) VALUES ('admin', '123')")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN signature TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN background_url TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE messages ADD COLUMN reply_to_id INTEGER")

	// 扩展状态表（用于持久化扩展开关）
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS extensions (
		key TEXT PRIMARY KEY,
		enabled INTEGER DEFAULT 0
	);`)
	if err != nil {
		log.Fatalf("创建extensions表失败: %v", err)
	}
	// 默认扩展列表与初始值
	_, _ = db.Exec("INSERT OR IGNORE INTO extensions (key, enabled) VALUES ('ai_chat', 0)")
	_, _ = db.Exec("INSERT OR IGNORE INTO extensions (key, enabled) VALUES ('secure_ws', 0)")
	// AI 配置表（保存 provider 与 keys 的 JSON）
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS ai_config (
		id TEXT PRIMARY KEY,
		value TEXT
	);`)
	if err != nil {
		log.Fatalf("创建ai_config表失败: %v", err)
	}
	// 初始化默认配置
	_, _ = db.Exec("INSERT OR IGNORE INTO ai_config (id, value) VALUES ('default', '{}')")
}

// AI 配置操作
func getAIConfigFromDB() (map[string]interface{}, error) {
	var raw string
	err := db.QueryRow("SELECT value FROM ai_config WHERE id = 'default'").Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{}, nil
		}
		return nil, err
	}
	var out map[string]interface{}
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		out = map[string]interface{}{}
	}
	return out, nil
}

func setAIConfigInDB(cfg map[string]interface{}) error {
	b, _ := json.Marshal(cfg)
	_, err := db.Exec("INSERT OR REPLACE INTO ai_config (id, value) VALUES ('default', ?)", string(b))
	if err == nil {
		broadcastAIConfigUpdate()
	}
	return err
}

func broadcastAIConfigUpdate() {
	cfg, err := getAIConfigFromDB()
	if err != nil {
		return
	}
	payload := map[string]interface{}{"type": "ai_config_update", "config": cfg}
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	for _, conn := range clients {
		_ = conn.WriteJSON(payload)
	}
}

func getIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if ip == "::1" {
		return "127.0.0.1"
	}
	return ip
}

func generateToken() string {
	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	return hex.EncodeToString(randBytes)
}

func saveSessionToken(username, token string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO session_tokens (token, username, created_at) VALUES (?, ?, ?)", token, username, time.Now().Unix())
	return err
}

func getUsernameByToken(token string) (string, error) {
	var username string
	err := db.QueryRow("SELECT username FROM session_tokens WHERE token = ?", token).Scan(&username)
	return username, err
}

func getTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// 扩展状态数据库操作
func getExtensionsFromDB() (map[string]bool, error) {
	rows, err := db.Query("SELECT key, enabled FROM extensions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]bool)
	for rows.Next() {
		var k string
		var e int
		_ = rows.Scan(&k, &e)
		res[k] = e != 0
	}
	return res, nil
}

func setExtensionInDB(key string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := db.Exec("INSERT OR REPLACE INTO extensions (key, enabled) VALUES (?, ?)", key, val)
	if err == nil {
		// 广播给所有在线客户端
		broadcastExtensionsUpdate()
	}
	return err
}

func broadcastExtensionsUpdate() {
	st, err := getExtensionsFromDB()
	if err != nil {
		return
	}
	payload := map[string]interface{}{"type": "extensions_update", "extensions": st}
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	for _, conn := range clients {
		_ = conn.WriteJSON(payload)
	}
}

func checkAdminSecret(r *http.Request) bool {
	secret := r.URL.Query().Get("secret")
	if secret == adminSecret {
		return true
	}
	if r.Method == http.MethodPost {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if s, ok := body["secret"].(string); ok && s == adminSecret {
			return true
		}
	}
	return false
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	clientIP := getIP(r)
	var authenticatedUser string

	defer func() {
		conn.Close()
		if authenticatedUser != "" {
			stateMutex.Lock()
			delete(clients, authenticatedUser)
			stateMutex.Unlock()
		}
	}()

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(msgBytes, &payload); err != nil {
			continue
		}

		action, _ := payload["action"].(string)

		switch action {
		case "login", "register":
			user, _ := payload["username"].(string)
			pwd, _ := payload["password"].(string)
			if user == "" || pwd == "" {
				conn.WriteJSON(map[string]string{"type": "auth_err", "content": "参数不能为空"})
				continue
			}

			var dbPwd, dbAvatar string
			err := db.QueryRow("SELECT password, avatar_url FROM users WHERE username = ?", user).Scan(&dbPwd, &dbAvatar)

			if action == "register" {
				if err == nil {
					conn.WriteJSON(map[string]string{"type": "auth_err", "content": "该账号已被注册"})
					continue
				}
				_, err = db.Exec("INSERT INTO users (username, password, last_ip) VALUES (?, ?, ?)", user, pwd, clientIP)
				if err != nil {
					conn.WriteJSON(map[string]string{"type": "auth_err", "content": "注册失败，请稍后再试"})
					continue
				}
			} else { // 登录
				if err == sql.ErrNoRows || dbPwd != pwd {
					conn.WriteJSON(map[string]string{"type": "auth_err", "content": "账号或密码错误"})
					continue
				}
				_, _ = db.Exec("UPDATE users SET last_ip = ? WHERE username = ?", clientIP, user)
			}

			stateMutex.Lock()
			authenticatedUser = user
			clients[user] = conn
			stateMutex.Unlock()

			token := generateToken()
			_ = saveSessionToken(user, token)
			conn.WriteJSON(map[string]string{"type": "auth_ok", "username": user, "token": token, "avatar_url": dbAvatar})

		case "resume":
			if authenticatedUser != "" {
				continue
			}
			token, _ := payload["token"].(string)
			if token == "" {
				conn.WriteJSON(map[string]string{"type": "auth_err", "content": "凭证不能为空"})
				continue
			}
			user, err := getUsernameByToken(token)
			if err != nil {
				conn.WriteJSON(map[string]string{"type": "auth_err", "content": "无效的登录凭证"})
				continue
			}
			var dbAvatar string
			if err := db.QueryRow("SELECT avatar_url FROM users WHERE username = ?", user).Scan(&dbAvatar); err != nil {
				conn.WriteJSON(map[string]string{"type": "auth_err", "content": "用户不存在"})
				continue
			}
			stateMutex.Lock()
			authenticatedUser = user
			clients[user] = conn
			stateMutex.Unlock()
			conn.WriteJSON(map[string]string{"type": "auth_ok", "username": user, "token": token, "avatar_url": dbAvatar})
			sendSyncData(user)

		case "sync":
			if authenticatedUser == "" {
				continue
			}
			sendSyncData(authenticatedUser)

		case "update_avatar":
			if authenticatedUser == "" {
				continue
			}
			content, _ := payload["content"].(string)
			_, err := db.Exec("UPDATE users SET avatar_url = ? WHERE username = ?", content, authenticatedUser)
			if err == nil {
				conn.WriteJSON(map[string]string{"type": "avatar_ok", "avatar_url": content})
			}

		case "add_friend":
			if authenticatedUser == "" {
				continue
			}
			target, _ := payload["target_user"].(string)
			if target == authenticatedUser {
				continue
			}

			var dummy string
			err := db.QueryRow("SELECT username FROM users WHERE username = ?", target).Scan(&dummy)
			if err == sql.ErrNoRows {
				continue
			}

			_, _ = db.Exec("INSERT OR IGNORE INTO friends (username, friend_username) VALUES (?, ?)", authenticatedUser, target)
			_, _ = db.Exec("INSERT OR IGNORE INTO friends (username, friend_username) VALUES (?, ?)", target, authenticatedUser)

			sendSyncData(authenticatedUser)
			stateMutex.RLock()
			if _, online := clients[target]; online {
				stateMutex.RUnlock()
				sendSyncData(target)
			} else {
				stateMutex.RUnlock()
			}

		case "delete_friend":
			if authenticatedUser == "" {
				continue
			}
			target, _ := payload["target_user"].(string)
			if target == "" || target == authenticatedUser {
				continue
			}

			_, _ = db.Exec("DELETE FROM friends WHERE username = ? AND friend_username = ?", authenticatedUser, target)
			_, _ = db.Exec("DELETE FROM friends WHERE username = ? AND friend_username = ?", target, authenticatedUser)

			conn.WriteJSON(map[string]string{"type": "delete_friend_ok", "target_user": target})
			sendSyncData(authenticatedUser)
			stateMutex.RLock()
			_, online := clients[target]
			stateMutex.RUnlock()
			if online {
				sendSyncData(target)
			}

		case "create_group":
			if authenticatedUser == "" {
				continue
			}
			gName, _ := payload["group_name"].(string)
			membersInter, _ := payload["members"].([]interface{})

			var members []string
			for _, m := range membersInter {
				if s, ok := m.(string); ok {
					members = append(members, s)
				}
			}
			members = append(members, authenticatedUser)

			if len(members) < 3 || gName == "" {
				conn.WriteJSON(map[string]string{"type": "create_group_err", "content": "群组创建不满足基本条件"})
				continue
			}

			// 写入群组
			res, err := db.Exec("INSERT INTO groups (name, owner) VALUES (?, ?)", gName, authenticatedUser)
			if err != nil {
				continue
			}
			gID, _ := res.LastInsertId()

			// 写入群成员
			for _, m := range members {
				_, _ = db.Exec("INSERT INTO group_members (group_id, username) VALUES (?, ?)", gID, m)
			}

			conn.WriteJSON(map[string]interface{}{
				"type":      "create_group_ok",
				"target_id": strconv.FormatInt(gID, 10),
				"content":   gName,
			})

			stateMutex.RLock()
			for _, m := range members {
				if m != authenticatedUser {
					if _, online := clients[m]; online {
						sendSyncData(m)
					}
				}
			}
			stateMutex.RUnlock()

		case "msg":
			if authenticatedUser == "" {
				continue
			}

			stateMutex.RLock()
			isMuted := globalMute
			stateMutex.RUnlock()
			if isMuted {
				continue
			}

			tType, _ := payload["target_type"].(string)
			tID, _ := payload["target_id"].(string)
			content, _ := payload["content"].(string)
			replyToIDFloat, _ := payload["reply_to_id"].(float64)

			var avatar string
			_ = db.QueryRow("SELECT avatar_url FROM users WHERE username = ?", authenticatedUser).Scan(&avatar)

			var replyToID *int64
			if replyToIDFloat > 0 {
				id := int64(replyToIDFloat)
				replyToID = &id
			}

			var err error
			var res sql.Result
			if replyToID != nil {
				res, err = db.Exec(`INSERT INTO messages (target_type, target_id, sender, content, timestamp, avatar_url, reply_to_id) 
					VALUES (?, ?, ?, ?, ?, ?, ?)`, tType, tID, authenticatedUser, content, time.Now().Unix(), avatar, *replyToID)
			} else {
				res, err = db.Exec(`INSERT INTO messages (target_type, target_id, sender, content, timestamp, avatar_url) 
					VALUES (?, ?, ?, ?, ?, ?)`, tType, tID, authenticatedUser, content, time.Now().Unix(), avatar)
			}
			if err != nil {
				continue
			}
			msgID, _ := res.LastInsertId()

			msg := Message{
				ID:         msgID,
				TargetType: tType,
				TargetID:   tID,
				Sender:     authenticatedUser,
				Content:    content,
				Timestamp:  time.Now().Unix(),
				AvatarURL:  avatar,
				ReplyToID:  replyToID,
			}

			// 如果是回复消息，查询被回复的消息信息
			if replyToID != nil && *replyToID > 0 {
				var repliedMsg Message
				err := db.QueryRow(`SELECT id, sender, content, avatar_url FROM messages WHERE id = ?`, *replyToID).
					Scan(&repliedMsg.ID, &repliedMsg.Sender, &repliedMsg.Content, &repliedMsg.AvatarURL)
				if err == nil {
					msg.ReplyToMsg = &repliedMsg
				}
			}

			broadcastMessage(msg)

		case "withdraw_message":
			if authenticatedUser == "" {
				continue
			}
			messageIDFloat, ok := payload["message_id"].(float64)
			if !ok {
				continue
			}
			messageID := int64(messageIDFloat)

			var targetType, targetID, sender string
			err := db.QueryRow("SELECT target_type, target_id, sender FROM messages WHERE id = ?", messageID).Scan(&targetType, &targetID, &sender)
			if err != nil || sender != authenticatedUser {
				conn.WriteJSON(map[string]string{"type": "withdraw_message_err", "content": "仅允许撤回自己的消息"})
				continue
			}

			_, err = db.Exec("DELETE FROM messages WHERE id = ?", messageID)
			if err != nil {
				conn.WriteJSON(map[string]string{"type": "withdraw_message_err", "content": "撤回失败，请稍后重试"})
				continue
			}

			conn.WriteJSON(map[string]interface{}{"type": "withdraw_message_ok", "message_id": messageID, "target_type": targetType, "target_id": targetID})
			broadcastWithdraw(targetType, targetID, messageID, authenticatedUser)

		case "rename_group":
			if authenticatedUser == "" {
				continue
			}
			groupIDStr, _ := payload["group_id"].(string)
			newName, _ := payload["new_name"].(string)
			if groupIDStr == "" || newName == "" {
				conn.WriteJSON(map[string]string{"type": "rename_group_err", "content": "参数不能为空"})
				continue
			}

			// 验证用户是否为群主
			var owner string
			err := db.QueryRow("SELECT owner FROM groups WHERE id = ?", groupIDStr).Scan(&owner)
			if err != nil || owner != authenticatedUser {
				conn.WriteJSON(map[string]string{"type": "rename_group_err", "content": "只有群主才能重命名群聊"})
				continue
			}

			// 更新群名
			_, err = db.Exec("UPDATE groups SET name = ? WHERE id = ?", newName, groupIDStr)
			if err != nil {
				conn.WriteJSON(map[string]string{"type": "rename_group_err", "content": "更新失败"})
				continue
			}

			conn.WriteJSON(map[string]interface{}{"type": "rename_group_ok", "group_id": groupIDStr, "new_name": newName})

			// 通知群成员重新同步
			gID, _ := strconv.Atoi(groupIDStr)
			rows, _ := db.Query("SELECT username FROM group_members WHERE group_id = ?", gID)
			if rows != nil {
				for rows.Next() {
					var member string
					_ = rows.Scan(&member)
					sendSyncData(member)
				}
				rows.Close()
			}

		case "publish_announcement":
			if authenticatedUser == "" {
				continue
			}
			groupIDStr, _ := payload["group_id"].(string)
			announcement, _ := payload["announcement"].(string)
			if groupIDStr == "" || announcement == "" {
				conn.WriteJSON(map[string]string{"type": "publish_announcement_err", "content": "参数不能为空"})
				continue
			}

			// 验证用户是否为群主
			var owner string
			err := db.QueryRow("SELECT owner FROM groups WHERE id = ?", groupIDStr).Scan(&owner)
			if err != nil || owner != authenticatedUser {
				conn.WriteJSON(map[string]string{"type": "publish_announcement_err", "content": "只有群主才能发布公告"})
				continue
			}

			conn.WriteJSON(map[string]interface{}{"type": "publish_announcement_ok", "group_id": groupIDStr})

			// 发送系统公告消息到群聊
			var avatar string
			_ = db.QueryRow("SELECT avatar_url FROM users WHERE username = ?", authenticatedUser).Scan(&avatar)

			res, err := db.Exec(`INSERT INTO messages (target_type, target_id, sender, content, timestamp, avatar_url) 
				VALUES (?, ?, ?, ?, ?, ?)`, "group", groupIDStr, "📢 群公告", announcement, time.Now().Unix(), avatar)
			if err == nil {
				msgID, _ := res.LastInsertId()
				msg := Message{
					ID:         msgID,
					TargetType: "group",
					TargetID:   groupIDStr,
					Sender:     "📢 群公告",
					Content:    announcement,
					Timestamp:  time.Now().Unix(),
					AvatarURL:  avatar,
				}
				broadcastMessage(msg)
			}

		case "disband_group":
			if authenticatedUser == "" {
				continue
			}
			groupIDStr, _ := payload["group_id"].(string)
			if groupIDStr == "" {
				conn.WriteJSON(map[string]string{"type": "disband_group_err", "content": "群聊ID不能为空"})
				continue
			}

			// 验证用户是否为群主
			var owner string
			err := db.QueryRow("SELECT owner FROM groups WHERE id = ?", groupIDStr).Scan(&owner)
			if err != nil || owner != authenticatedUser {
				conn.WriteJSON(map[string]string{"type": "disband_group_err", "content": "只有群主才能解散群聊"})
				continue
			}

			gID, _ := strconv.Atoi(groupIDStr)

			// 获取群成员
			rows, _ := db.Query("SELECT username FROM group_members WHERE group_id = ?", gID)
			var members []string
			if rows != nil {
				for rows.Next() {
					var member string
					_ = rows.Scan(&member)
					members = append(members, member)
				}
				rows.Close()
			}

			// 删除群聊、成员、消息
			_, _ = db.Exec("DELETE FROM groups WHERE id = ?", gID)
			_, _ = db.Exec("DELETE FROM group_members WHERE group_id = ?", gID)
			_, _ = db.Exec("DELETE FROM messages WHERE target_type = 'group' AND target_id = ?", groupIDStr)

			conn.WriteJSON(map[string]interface{}{"type": "disband_group_ok", "group_id": groupIDStr})

			// 通知所有成员重新同步
			stateMutex.RLock()
			for _, member := range members {
				if _, online := clients[member]; online {
					sendSyncData(member)
				}
			}
			stateMutex.RUnlock()

		case "quit_group":
			if authenticatedUser == "" {
				continue
			}
			groupIDStr, _ := payload["group_id"].(string)
			if groupIDStr == "" {
				conn.WriteJSON(map[string]string{"type": "quit_group_err", "content": "群聊ID不能为空"})
				continue
			}

			gID, _ := strconv.Atoi(groupIDStr)

			// 删除用户从群聊中
			_, err := db.Exec("DELETE FROM group_members WHERE group_id = ? AND username = ?", gID, authenticatedUser)
			if err != nil {
				conn.WriteJSON(map[string]string{"type": "quit_group_err", "content": "退出失败"})
				continue
			}

			conn.WriteJSON(map[string]interface{}{"type": "quit_group_ok", "group_id": groupIDStr})
			sendSyncData(authenticatedUser)

		case "add_member_to_group":
			if authenticatedUser == "" {
				continue
			}
			groupIDStr, _ := payload["group_id"].(string)
			newMembersInter, _ := payload["members"].([]interface{})
			if groupIDStr == "" || len(newMembersInter) == 0 {
				conn.WriteJSON(map[string]string{"type": "add_member_err", "content": "参数不能为空"})
				continue
			}

			// 验证用户是否为群主
			var owner string
			err := db.QueryRow("SELECT owner FROM groups WHERE id = ?", groupIDStr).Scan(&owner)
			if err != nil || owner != authenticatedUser {
				conn.WriteJSON(map[string]string{"type": "add_member_err", "content": "只有群主才能添加成员"})
				continue
			}

			gID, _ := strconv.Atoi(groupIDStr)
			var addedCount int

			// 添加新成员
			for _, m := range newMembersInter {
				if s, ok := m.(string); ok {
					// 检查该用户是否存在
					var dummy string
					err := db.QueryRow("SELECT username FROM users WHERE username = ?", s).Scan(&dummy)
					if err == nil {
						// 添加成员
						res, err := db.Exec("INSERT OR IGNORE INTO group_members (group_id, username) VALUES (?, ?)", gID, s)
						if err == nil {
							affected, _ := res.RowsAffected()
							if affected > 0 {
								addedCount++
								// 通知新成员
								sendSyncData(s)
							}
						}
					}
				}
			}

			conn.WriteJSON(map[string]interface{}{"type": "add_member_ok", "group_id": groupIDStr, "added": addedCount})
		}
	}
}

func sendSyncData(username string) {
	stateMutex.RLock()
	conn, online := clients[username]
	stateMutex.RUnlock()
	if !online {
		return
	}

	// 查询好友列表
	rows, err := db.Query("SELECT friend_username FROM friends WHERE username = ?", username)
	var friends []string
	if err == nil {
		for rows.Next() {
			var f string
			_ = rows.Scan(&f)
			friends = append(friends, f)
		}
		rows.Close()
	}

	gRows, err := db.Query(`SELECT g.id, g.name, g.owner FROM groups g 
		JOIN group_members gm ON g.id = gm.group_id WHERE gm.username = ?`, username)
	syncGroups := make([]map[string]interface{}, 0)
	if err == nil {
		for gRows.Next() {
			var id int
			var name, owner string
			_ = gRows.Scan(&id, &name, &owner)
			syncGroups = append(syncGroups, map[string]interface{}{
				"id":    id,
				"name":  name,
				"owner": owner,
			})
		}
		gRows.Close()
	}

	_ = conn.WriteJSON(map[string]interface{}{
		"type":    "sync_data",
		"friends": friends,
		"groups":  syncGroups,
	})
}

func broadcastMessage(msg Message) {
	stateMutex.RLock()
	defer stateMutex.RUnlock()

	msgWithType := map[string]interface{}{
		"type":        "msg",
		"id":          msg.ID,
		"target_type": msg.TargetType,
		"target_id":   msg.TargetID,
		"sender":      msg.Sender,
		"content":     msg.Content,
		"timestamp":   msg.Timestamp,
		"avatar_url":  msg.AvatarURL,
	}

	// 如果是回复消息，添加回复信息
	if msg.ReplyToID != nil {
		msgWithType["reply_to_id"] = *msg.ReplyToID
		if msg.ReplyToMsg != nil {
			msgWithType["reply_to_msg"] = map[string]interface{}{
				"id":         msg.ReplyToMsg.ID,
				"sender":     msg.ReplyToMsg.Sender,
				"content":    msg.ReplyToMsg.Content,
				"avatar_url": msg.ReplyToMsg.AvatarURL,
			}
		}
	}

	switch msg.TargetType {
	case "public":
		for _, conn := range clients {
			_ = conn.WriteJSON(msgWithType)
		}
	case "group":
		gID, _ := strconv.Atoi(msg.TargetID)
		rows, err := db.Query("SELECT username FROM group_members WHERE group_id = ?", gID)
		if err == nil {
			for rows.Next() {
				var member string
				_ = rows.Scan(&member)
				if conn, online := clients[member]; online {
					_ = conn.WriteJSON(msgWithType)
				}
			}
			rows.Close()
		}
	case "private":
		if conn, online := clients[msg.Sender]; online {
			_ = conn.WriteJSON(msgWithType)
		}
		if msg.Sender != msg.TargetID {
			if conn, online := clients[msg.TargetID]; online {
				_ = conn.WriteJSON(msgWithType)
			}
		}
	}
}

func broadcastWithdraw(targetType, targetID string, messageID int64, sender string) {
	stateMutex.RLock()
	defer stateMutex.RUnlock()

	payload := map[string]interface{}{
		"type":        "withdraw_message",
		"message_id":  messageID,
		"target_type": targetType,
		"target_id":   targetID,
	}

	switch targetType {
	case "public":
		for _, conn := range clients {
			_ = conn.WriteJSON(payload)
		}
	case "group":
		gID, _ := strconv.Atoi(targetID)
		rows, err := db.Query("SELECT username FROM group_members WHERE group_id = ?", gID)
		if err == nil {
			for rows.Next() {
				var member string
				_ = rows.Scan(&member)
				if conn, online := clients[member]; online {
					_ = conn.WriteJSON(payload)
				}
			}
			rows.Close()
		}
	case "private":
		if conn, online := clients[targetID]; online {
			_ = conn.WriteJSON(payload)
		}
		if sender != targetID {
			if conn, online := clients[sender]; online {
				_ = conn.WriteJSON(payload)
			}
		}
	}
}

// 公共接口：获取扩展状态（允许匿名获取）
func handleGetExtensions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	st, err := getExtensionsFromDB()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

// 管理接口：设置 AI 配置（provider + keys），需要 secret
func handleAdminSetAIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if !checkAdminSecret(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		cfg, err := getAIConfigFromDB()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// 读取请求体并可复用，避免 checkAdminSecret 消耗 Body
	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if !checkAdminSecret(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 仅存储 provider 与 keys
	cfg := map[string]interface{}{}
	if p, ok := body["provider"].(string); ok {
		cfg["provider"] = p
	}
	if k, ok := body["keys"].(map[string]interface{}); ok {
		cfg["keys"] = k
	}
	if err := setAIConfigInDB(cfg); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// 公共接口：获取 AI 配置（不返回密钥），允许客户端查看 provider
func handleGetAIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	cfg, err := getAIConfigFromDB()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// 不暴露 keys
	out := map[string]interface{}{}
	if p, ok := cfg["provider"]; ok {
		out["provider"] = p
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// 管理接口：设置扩展状态（需 secret）
func handleAdminSetExtensions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkAdminSecret(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var body struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if body.Key == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := setExtensionInDB(body.Key, body.Enabled); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// 返回最新状态
	st, _ := getExtensionsFromDB()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

// AI 占位聊天接口
func handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// 检查扩展是否启用
	st, err := getExtensionsFromDB()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !st["ai_chat"] {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "AI 扩展未启用"})
		return
	}
	var body struct {
		prompt string `json:"prompt"`
	}
	// 先以通用解析
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	prompt, _ := payload["prompt"].(string)
	if prompt == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 读取 AI 配置并根据 provider 调用对应实现
	cfg, _ := getAIConfigFromDB()
	provider, _ := cfg["provider"].(string)
	keysMap := map[string]string{}
	if k, ok := cfg["keys"].(map[string]interface{}); ok {
		for kk, vv := range k {
			if s, ok2 := vv.(string); ok2 {
				keysMap[kk] = s
			}
		}
	}

	var reply string
	var callErr error
	switch strings.ToLower(provider) {
	case "deepseek":
		reply, callErr = callDeepseek(prompt, keysMap)
	case "gemini":
		reply, callErr = callGemini(prompt, keysMap)
	case "claude":
		reply, callErr = callClaude(prompt, keysMap)
	default:
		// 默认占位回复
		reply = fmt.Sprintf("[AI 占位回复] 我收到了你的问题：%s", prompt)
	}

	if callErr != nil {
		// 返回占位并附带错误
		reply = fmt.Sprintf("[AI 错误] %v — 原始输入：%s", callErr, prompt)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"reply": reply})
}

// provider implementations (占位或示例实现)
func callDeepseek(prompt string, keys map[string]string) (string, error) {
	// 示例: 如果配置了 key，尝试调用 Deepseek API（占位 URL）
	apiKey := keys["deepseek_key"]
	if apiKey == "" {
		return "", fmt.Errorf("Deepseek key 未配置")
	}
	// 这里写入实际 API 调用逻辑，下面为占位回显
	return fmt.Sprintf("[Deepseek] 回复: %s", prompt), nil
}

func callGemini(prompt string, keys map[string]string) (string, error) {
	apiKey := keys["gemini_key"]
	if apiKey == "" {
		return "", fmt.Errorf("Gemini key 未配置")
	}
	// 占位实现
	return fmt.Sprintf("[Gemini] 回复: %s", prompt), nil
}

func callClaude(prompt string, keys map[string]string) (string, error) {
	apiKey := keys["claude_key"]
	if apiKey == "" {
		return "", fmt.Errorf("Claude key 未配置")
	}
	// 占位实现
	return fmt.Sprintf("[Claude] 回复: %s", prompt), nil
}

// 获取消息历史接口
func handleGetMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}
	targetType := r.URL.Query().Get("type")
	targetID := r.URL.Query().Get("id")
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "100"
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	var rows *sql.Rows
	if targetType == "private" {
		currentUser := ""
		token := getTokenFromHeader(r)
		if token != "" {
			currentUser, _ = getUsernameByToken(token)
		}
		if currentUser == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		rows, err = db.Query(`SELECT id, target_type, target_id, sender, content, timestamp, avatar_url, reply_to_id
			FROM messages
			WHERE target_type = 'private' AND ((target_id = ? AND sender = ?) OR (target_id = ? AND sender = ?))
			ORDER BY id DESC LIMIT ?`, targetID, currentUser, currentUser, targetID, limit)
	} else {
		rows, err = db.Query(`SELECT id, target_type, target_id, sender, content, timestamp, avatar_url, reply_to_id
			FROM messages WHERE target_type = ? AND target_id = ?
			ORDER BY id DESC LIMIT ?`, targetType, targetID, limit)
	}

	var msgs []Message
	if err == nil {
		for rows.Next() {
			var m Message
			var replyToID *int64
			_ = rows.Scan(&m.ID, &m.TargetType, &m.TargetID, &m.Sender, &m.Content, &m.Timestamp, &m.AvatarURL, &replyToID)
			m.ReplyToID = replyToID

			// 如果有回复，获取被回复消息的详细信息
			if replyToID != nil && *replyToID > 0 {
				var repliedMsg Message
				_ = db.QueryRow(`SELECT id, sender, content, avatar_url FROM messages WHERE id = ?`, *replyToID).
					Scan(&repliedMsg.ID, &repliedMsg.Sender, &repliedMsg.Content, &repliedMsg.AvatarURL)
				m.ReplyToMsg = &repliedMsg
			}

			msgs = append(msgs, m)
		}
		rows.Close()
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}

func handleGroupMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}
	// 解析 /api/group/{groupId}/members
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[len(parts)-1] != "members" {
		return
	}
	groupIDStr := parts[len(parts)-2]

	gID, err := strconv.Atoi(groupIDStr)
	if err != nil {
		return
	}

	// 查询群成员和群主
	var owner string
	_ = db.QueryRow("SELECT owner FROM groups WHERE id = ?", gID).Scan(&owner)

	rows, err := db.Query("SELECT username FROM group_members WHERE group_id = ?", gID)
	type MemberInfo struct {
		Username string `json:"username"`
		IsOwner  bool   `json:"is_owner"`
	}
	var members []MemberInfo
	if err == nil {
		for rows.Next() {
			var member string
			_ = rows.Scan(&member)
			members = append(members, MemberInfo{
				Username: member,
				IsOwner:  member == owner,
			})
		}
		rows.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"members": members,
	})
}

func handleGetOnlineUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}
	stateMutex.RLock()
	count := len(clients)
	stateMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{
		"online_count": count,
	})
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "无效的文件"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	newFileName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(randBytes), ext)
	savePath := filepath.Join("./uploads", newFileName)

	out, err := os.Create(savePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, file)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"url": "/uploads/" + newFileName})
}

func handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	var req map[string]string
	_ = json.NewDecoder(r.Body).Decode(&req)

	username := req["username"]
	newPassword := req["password"]

	res, err := db.Exec("UPDATE users SET password = ? WHERE username = ?", newPassword, username)
	affected, _ := res.RowsAffected()

	w.Header().Set("Content-Type", "application/json")
	if err == nil && affected > 0 {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	} else {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "未找到对应的用户账号"})
	}
}

func handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if !checkAdminSecret(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	rows, err := db.Query("SELECT username, last_ip FROM users")
	type AdminUserView struct {
		IsOnline bool   `json:"is_online"`
		Username string `json:"username"`
		LastIP   string `json:"last_ip"`
	}
	var list []AdminUserView

	stateMutex.RLock()
	if err == nil {
		for rows.Next() {
			var u, ip string
			_ = rows.Scan(&u, &ip)
			_, online := clients[u]
			list = append(list, AdminUserView{IsOnline: online, Username: u, LastIP: ip})
		}
		rows.Close()
	}
	stateMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func handleAdminMessages(w http.ResponseWriter, r *http.Request) {
	if !checkAdminSecret(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	rows, err := db.Query("SELECT id, sender, content FROM messages ORDER BY id DESC")
	var list []AdminMessage
	if err == nil {
		for rows.Next() {
			var m AdminMessage
			_ = rows.Scan(&m.ID, &m.Sender, &m.Content)
			list = append(list, m)
		}
		rows.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	var req map[string]string
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req["secret"] != adminSecret {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	target := req["username"]
	stateMutex.Lock()
	if conn, online := clients[target]; online {
		_ = conn.WriteJSON(map[string]string{"type": "auth_err", "content": "您的账号已被管理员注销"})
		conn.Close()
		delete(clients, target)
	}
	stateMutex.Unlock()

	_, _ = db.Exec("DELETE FROM users WHERE username = ?", target)
	_, _ = db.Exec("DELETE FROM friends WHERE username = ? OR friend_username = ?", target, target)
	_, _ = db.Exec("DELETE FROM group_members WHERE username = ?", target)

	w.WriteHeader(http.StatusOK)
}

func handleAdminDeleteMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	var req map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req["secret"] != adminSecret {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	idFloat, _ := req["id"].(float64)
	_, _ = db.Exec("DELETE FROM messages WHERE id = ?", int64(idFloat))
	w.WriteHeader(http.StatusOK)
}

func handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	if !checkAdminSecret(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	stateMutex.RLock()
	m := globalMute
	stateMutex.RUnlock()
	_ = json.NewEncoder(w).Encode(map[string]bool{"global_mute": m})
}

func handleAdminToggleMute(w http.ResponseWriter, r *http.Request) {
	if !checkAdminSecret(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	stateMutex.Lock()
	globalMute = !globalMute
	m := globalMute
	stateMutex.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]bool{"global_mute": m})
}

func handleAdminBroadcast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	var req map[string]string
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req["secret"] != adminSecret {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	res, err := db.Exec(`INSERT INTO messages (target_type, target_id, sender, content, timestamp, avatar_url) 
		VALUES ('public', 'global', '📢 系统公告', ?, ?, '')`, req["content"], time.Now().Unix())
	if err != nil {
		return
	}
	msgID, _ := res.LastInsertId()

	msg := Message{
		ID:         msgID,
		TargetType: "public",
		TargetID:   "global",
		Sender:     "📢 系统公告",
		Content:    req["content"],
		Timestamp:  time.Now().Unix(),
	}
	broadcastMessage(msg)
	w.WriteHeader(http.StatusOK)
}

// --- 用户资料接口 ---

func handleUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}
	username := r.URL.Query().Get("username")
	token := getTokenFromHeader(r)
	if username == "" {
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		u, err := getUsernameByToken(token)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		username = u
	}

	var avatar, signature, background string
	err := db.QueryRow("SELECT avatar_url, signature, background_url FROM users WHERE username = ?", username).Scan(&avatar, &signature, &background)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"username":       username,
		"avatar_url":     avatar,
		"signature":      signature,
		"background_url": background,
	})
}

func handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	token := getTokenFromHeader(r)
	if token == "" {
		log.Printf("[DEBUG] handleUserUpdate: Authorization 头缺失或格式错误")
		log.Printf("[DEBUG] 收到的 Authorization 头: %s", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	user, err := getUsernameByToken(token)
	if err != nil {
		log.Printf("[DEBUG] handleUserUpdate: token 无效。错误: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req map[string]string
	_ = json.NewDecoder(r.Body).Decode(&req)

	// 更新签名
	if sig, ok := req["signature"]; ok {
		_, _ = db.Exec("UPDATE users SET signature = ? WHERE username = ?", sig, user)
	}

	// 更新用户名（需要迁移相关表）
	if newName, ok := req["username"]; ok && newName != "" && newName != user {
		// 检查是否已存在
		var exists string
		err := db.QueryRow("SELECT username FROM users WHERE username = ?", newName).Scan(&exists)
		if err != sql.ErrNoRows {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "username_exists"})
			return
		}

		tx, _ := db.Begin()
		_, _ = tx.Exec("UPDATE users SET username = ? WHERE username = ?", newName, user)
		_, _ = tx.Exec("UPDATE session_tokens SET username = ? WHERE username = ?", newName, user)
		_, _ = tx.Exec("UPDATE friends SET username = ? WHERE username = ?", newName, user)
		_, _ = tx.Exec("UPDATE friends SET friend_username = ? WHERE friend_username = ?", newName, user)
		_, _ = tx.Exec("UPDATE group_members SET username = ? WHERE username = ?", newName, user)
		_, _ = tx.Exec("UPDATE groups SET owner = ? WHERE owner = ?", newName, user)
		_, _ = tx.Exec("UPDATE messages SET sender = ? WHERE sender = ?", newName, user)
		_, _ = tx.Exec("UPDATE messages SET target_id = ? WHERE target_type = 'private' AND target_id = ?", newName, user)
		_ = tx.Commit()

		stateMutex.Lock()
		if conn, ok := clients[user]; ok {
			clients[newName] = conn
			delete(clients, user)
		}
		stateMutex.Unlock()
	}

	w.WriteHeader(http.StatusOK)
}

func handleUserAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	token := getTokenFromHeader(r)
	if token == "" {
		log.Printf("[DEBUG] handleUserAvatar: Authorization 头缺失或格式错误")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	user, err := getUsernameByToken(token)
	if err != nil {
		log.Printf("[DEBUG] handleUserAvatar: token 无效。错误: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	newFileName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(randBytes), ext)
	savePath := filepath.Join("./uploads", newFileName)
	out, err := os.Create(savePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, file)

	url := "/uploads/" + newFileName
	_, _ = db.Exec("UPDATE users SET avatar_url = ? WHERE username = ?", url, user)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": url})
}

func handleUserBackground(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	token := getTokenFromHeader(r)
	if token == "" {
		log.Printf("[DEBUG] handleUserBackground: Authorization 头缺失或格式错误")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	user, err := getUsernameByToken(token)
	if err != nil {
		log.Printf("[DEBUG] handleUserBackground: token 无效。错误: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	file, header, err := r.FormFile("background")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	newFileName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(randBytes), ext)
	savePath := filepath.Join("./uploads", newFileName)
	out, err := os.Create(savePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, file)

	url := "/uploads/" + newFileName
	_, _ = db.Exec("UPDATE users SET background_url = ? WHERE username = ?", url, user)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": url})
}
