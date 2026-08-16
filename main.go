package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
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
	"unicode/utf16"
	"unicode/utf8"

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
	db              *sql.DB
	clients         = make(map[string]*websocket.Conn)
	globalMute      = false
	stateMutex      sync.RWMutex
	adminSecret     = "admin666" // 默认管理员密码
	adminConfigPath = "config.txt"

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func main() {
	if err := os.MkdirAll("./uploads", 0755); err != nil {
		log.Fatalf("无法创建上传目录: %v", err)
	}
	if err := os.MkdirAll("./uploads/public", 0755); err != nil {
		log.Fatalf("无法创建公共上传目录: %v", err)
	}
	if err := os.MkdirAll("./uploads/private", 0755); err != nil {
		log.Fatalf("无法创建私有上传目录: %v", err)
	}

	// 确保 photos/uploads 结构存在，用于用户可见的背景/字体存放
	if err := os.MkdirAll("./photos/uploads/userpublic", 0755); err != nil {
		log.Fatalf("无法创建 photos 公共上传目录: %v", err)
	}

	loadAdminConfig()
	initDB()
	defer db.Close()

	http.Handle("/", http.FileServer(http.Dir("./")))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))
	http.Handle("/favicon.ico", http.FileServer(http.Dir("./photos/ico")))
	// serve user-uploaded public/private assets (backgrounds/fonts)
	// 静态托管改为受保护处理：公共目录仍然可公开访问，私有目录需要 token 验证
	http.HandleFunc("/photos/uploads/", handleProtectedUploads)

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/api/assets", handleAssets)
	http.HandleFunc("/api/user/info", handleUserInfo)
	http.HandleFunc("/api/user/update", handleUserUpdate)
	http.HandleFunc("/api/user/avatar", handleUserAvatar)
	http.HandleFunc("/api/user/background", handleUserBackground)
	http.HandleFunc("/api/user/font", handleUserFont)
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
	http.HandleFunc("/api/admin/change-password", handleAdminChangePassword)
	http.HandleFunc("/api/extensions", handleGetExtensions)
	http.HandleFunc("/api/admin/extensions", handleAdminSetExtensions)
	http.HandleFunc("/api/ai/chat", handleAIChat)
	http.HandleFunc("/api/admin/ai/config", handleAdminSetAIConfig)
	http.HandleFunc("/api/ai/config", handleGetAIConfig)
	http.HandleFunc("/api/admin/login", handleAdminLogin)
	http.HandleFunc("/api/admin/secure_ws", handleAdminSecureWS)
	http.HandleFunc("/api/admin/refresh-token", handleRefreshToken)

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

func loadAdminConfig() {
	password, err := readAdminPasswordFromConfig()
	if err != nil {
		log.Printf("读取管理员配置失败: %v", err)
		return
	}
	if password != "" {
		adminSecret = password
	}
}

func readAdminPasswordFromConfig() (string, error) {
	data, err := os.ReadFile(adminConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			content := "# 管理员密码配置\nadminpassword=admin666\n"
			if writeErr := os.WriteFile(adminConfigPath, []byte(content), 0644); writeErr != nil {
				return "", writeErr
			}
			fmt.Printf("[INFO] 首次启动，已生成配置文件 %s，请修改 adminpassword 项\n", adminConfigPath)
			return adminSecret, nil
		}
		return "", err
	}

	text, err := decodeConfigText(data)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "adminpassword=") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "adminpassword="))
			if value == "" {
				return adminSecret, nil
			}
			return value, nil
		}
	}

	if !strings.Contains(text, "adminpassword=") {
		updated := strings.TrimRight(text, "\n") + "\nadminpassword=" + adminSecret + "\n"
		if writeErr := os.WriteFile(adminConfigPath, []byte(updated), 0644); writeErr != nil {
			return "", writeErr
		}
	}
	return adminSecret, nil
}

func decodeConfigText(data []byte) (string, error) {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	if len(data) >= 2 {
		if data[0] == 0xFF && data[1] == 0xFE {
			return decodeUTF16LE(data[2:]), nil
		}
		if data[0] == 0xFE && data[1] == 0xFF {
			return decodeUTF16BE(data[2:]), nil
		}
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	return string(data), nil
}

func decodeUTF16LE(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	u16s := make([]uint16, len(data)/2)
	for i := 0; i < len(u16s); i++ {
		u16s[i] = uint16(data[i*2]) | uint16(data[i*2+1])<<8
	}
	return string(utf16.Decode(u16s))
}

func decodeUTF16BE(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	u16s := make([]uint16, len(data)/2)
	for i := 0; i < len(u16s); i++ {
		u16s[i] = uint16(data[i*2+1]) | uint16(data[i*2])<<8
	}
	return string(utf16.Decode(u16s))
}

func saveAdminPasswordToConfig(password string) error {
	data, err := os.ReadFile(adminConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			content := "# 管理员密码配置\nadminpassword=" + password + "\n"
			return os.WriteFile(adminConfigPath, []byte(content), 0644)
		}
		return err
	}

	text, err := decodeConfigText(data)
	if err != nil {
		return err
	}

	lines := strings.Split(text, "\n")
	updated := false
	for idx, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "adminpassword=") {
			lines[idx] = "adminpassword=" + password
			updated = true
			break
		}
	}
	if !updated {
		lines = append(lines, "adminpassword="+password)
	}

	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(adminConfigPath, []byte(out), 0644)
}

func getAdminPassword() string {
	password, err := readAdminPasswordFromConfig()
	if err != nil {
		return adminSecret
	}
	if password != "" {
		return password
	}
	return adminSecret
}

func verifyAdminPassword(password string) bool {
	if strings.EqualFold(password, getAdminPassword()) {
		return true
	}
	var dbPwd string
	if err := db.QueryRow("SELECT password FROM users WHERE username = ?", "admin").Scan(&dbPwd); err == nil {
		return strings.EqualFold(dbPwd, password)
	}
	return false
}

func handleAdminChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	if !checkAdminSecret(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req.OldPassword = strings.TrimSpace(req.OldPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.OldPassword == "" || req.NewPassword == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !verifyAdminPassword(req.OldPassword) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := saveAdminPasswordToConfig(req.NewPassword); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	adminSecret = req.NewPassword

	res, err := db.Exec("UPDATE users SET password = ? WHERE username = ?", req.NewPassword, "admin")
	if err == nil {
		if affected, _ := res.RowsAffected(); affected == 0 {
			_, _ = db.Exec("INSERT OR IGNORE INTO users (username, password) VALUES (?, ?)", "admin", req.NewPassword)
		}
	}

	newToken := generateToken()
	_ = saveSessionToken("admin", newToken)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "token": newToken})
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

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS friend_requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_user TEXT NOT NULL,
		to_user TEXT NOT NULL,
		message TEXT DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		created_at INTEGER DEFAULT (strftime('%s','now')),
		reviewed_at INTEGER DEFAULT 0,
		reviewed_by TEXT DEFAULT ''
	);`)
	if err != nil {
		log.Fatalf("创建friend_requests表失败: %v", err)
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
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN font_url TEXT DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE messages ADD COLUMN reply_to_id INTEGER")

	// 扩展状态表（用于持久化扩展开关）
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS extensions (
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

	// secure_ws key storage
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS secure_ws (
		id TEXT PRIMARY KEY,
		value TEXT
	);`)
	if err != nil {
		log.Fatalf("创建secure_ws表失败: %v", err)
	}
	_, _ = db.Exec("INSERT OR IGNORE INTO secure_ws (id, value) VALUES ('default', '')")
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

// secure_ws key operations
func getSecureWSKeyFromDB() (string, error) {
	var raw string
	err := db.QueryRow("SELECT value FROM secure_ws WHERE id = 'default'").Scan(&raw)
	if err != nil {
		return "", err
	}
	return raw, nil
}

func setSecureWSKeyInDB(val string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO secure_ws (id, value) VALUES ('default', ?)", val)
	if err == nil {
		// broadcast optional update
		payload := map[string]interface{}{"type": "secure_ws_update", "configured": val != ""}
		stateMutex.RLock()
		for _, conn := range clients {
			_ = conn.WriteJSON(payload)
		}
		stateMutex.RUnlock()
	}
	return err
}

// decrypt secure payload (expects base64 of nonce(12)+ciphertext)
func decryptSecurePayload(b64 string, keyStr string) (map[string]interface{}, error) {
	if b64 == "" || keyStr == "" {
		return nil, fmt.Errorf("empty payload or key")
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	k := []byte(keyStr)
	if len(k) != 16 && len(k) != 24 && len(k) != 32 {
		return nil, fmt.Errorf("invalid key length")
	}
	if len(data) < 12 {
		return nil, fmt.Errorf("payload too short")
	}
	nonce := data[:12]
	ct := data[12:]
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
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
	var createdAt int64
	err := db.QueryRow("SELECT username, created_at FROM session_tokens WHERE token = ?", token).Scan(&username, &createdAt)
	if err != nil {
		return "", err
	}
	// token expiry: 30 days
	if time.Now().Unix()-createdAt > int64(30*24*3600) {
		return "", fmt.Errorf("token expired")
	}
	return username, nil
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
	// 支持 Bearer token 或 query/body admin_token，兼容旧 secret
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			token := strings.TrimSpace(parts[1])
			if token != "" {
				if user, err := getUsernameByToken(token); err == nil && user == "admin" {
					return true
				}
			}
		}
	}
	// query param admin_token
	token := r.URL.Query().Get("admin_token")
	if token != "" {
		if user, err := getUsernameByToken(token); err == nil && user == "admin" {
			return true
		}
	}
	// 如果是 POST，尝试从请求体读取 admin_token（不消耗 Body）
	if r.Method == http.MethodPost {
		raw, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(raw))
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		if s, ok := body["admin_token"].(string); ok && s != "" {
			if user, err := getUsernameByToken(s); err == nil && user == "admin" {
				return true
			}
		}
		// restore body for caller
		r.Body = io.NopCloser(bytes.NewReader(raw))
	}
	// fallback to legacy secret param
	secret := r.URL.Query().Get("secret")
	if strings.EqualFold(secret, getAdminPassword()) {
		return true
	}
	return false
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] upgrade failed: %v, headers=%v", err, r.Header)
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

		// 如果启用了 secure_ws 并且为加密消息，尝试解密（PoC）
		if secureEnabled, _ := getExtensionsFromDB(); secureEnabled["secure_ws"] {
			if sp, ok := payload["secure_payload"].(string); ok && sp != "" {
				if key, err := getSecureWSKeyFromDB(); err == nil && key != "" {
					if decrypted, err := decryptSecurePayload(sp, key); err == nil {
						payload = decrypted
					}
				}
			}
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
			message, _ := payload["message"].(string)
			target = strings.TrimSpace(target)
			message = strings.TrimSpace(message)
			if target == "" || target == authenticatedUser {
				conn.WriteJSON(map[string]string{"type": "add_friend_err", "content": "好友账号不能为空，且不能添加自己"})
				continue
			}

			var dummy string
			err := db.QueryRow("SELECT username FROM users WHERE username = ?", target).Scan(&dummy)
			if err == sql.ErrNoRows {
				conn.WriteJSON(map[string]string{"type": "add_friend_err", "content": "目标账号不存在"})
				continue
			}

			var friendCount int
			err = db.QueryRow("SELECT COUNT(*) FROM friends WHERE username = ? AND friend_username = ?", authenticatedUser, target).Scan(&friendCount)
			if err == nil && friendCount > 0 {
				conn.WriteJSON(map[string]string{"type": "add_friend_err", "content": "你们已经是好友了"})
				continue
			}

			var requestID int
			var requestStatus string
			err = db.QueryRow(`SELECT id, status FROM friend_requests WHERE ((from_user = ? AND to_user = ?) OR (from_user = ? AND to_user = ?)) ORDER BY created_at DESC LIMIT 1`, authenticatedUser, target, target, authenticatedUser).Scan(&requestID, &requestStatus)
			if err == nil && requestStatus == "pending" {
				conn.WriteJSON(map[string]string{"type": "add_friend_err", "content": "该好友申请已发送，等待对方审核"})
				continue
			}
			if err == nil && requestStatus == "approved" {
				conn.WriteJSON(map[string]string{"type": "add_friend_err", "content": "你们已经通过好友验证"})
				continue
			}
			if err == nil && requestStatus == "rejected" {
				_, err = db.Exec("UPDATE friend_requests SET message = ?, status = 'pending', created_at = ?, reviewed_at = 0, reviewed_by = '' WHERE id = ?", message, time.Now().Unix(), requestID)
			} else if err == sql.ErrNoRows {
				_, err = db.Exec("INSERT INTO friend_requests (from_user, to_user, message, status, created_at) VALUES (?, ?, ?, 'pending', ?)", authenticatedUser, target, message, time.Now().Unix())
			}
			if err != nil {
				conn.WriteJSON(map[string]string{"type": "add_friend_err", "content": "发送好友申请失败，请稍后再试"})
				continue
			}

			conn.WriteJSON(map[string]string{"type": "add_friend_ok", "target_user": target, "content": "好友申请已发送，等待对方审核"})
			sendSyncData(authenticatedUser)
			stateMutex.RLock()
			if _, online := clients[target]; online {
				stateMutex.RUnlock()
				sendSyncData(target)
			} else {
				stateMutex.RUnlock()
			}

		case "review_friend_request":
			if authenticatedUser == "" {
				continue
			}
			requestIDFloat, _ := payload["request_id"].(float64)
			action, _ := payload["action"].(string)
			requestID := int(requestIDFloat)
			if requestID <= 0 || (action != "approve" && action != "reject") {
				continue
			}

			var fromUser, toUser, status string
			err := db.QueryRow("SELECT from_user, to_user, status FROM friend_requests WHERE id = ?", requestID).Scan(&fromUser, &toUser, &status)
			if err != nil || status != "pending" {
				continue
			}
			if toUser != authenticatedUser {
				continue
			}

			if action == "approve" {
				_, _ = db.Exec("INSERT OR IGNORE INTO friends (username, friend_username) VALUES (?, ?)", fromUser, toUser)
				_, _ = db.Exec("INSERT OR IGNORE INTO friends (username, friend_username) VALUES (?, ?)", toUser, fromUser)
				_, _ = db.Exec("UPDATE friend_requests SET status = 'approved', reviewed_at = ?, reviewed_by = ? WHERE id = ?", time.Now().Unix(), authenticatedUser, requestID)
			} else {
				_, _ = db.Exec("UPDATE friend_requests SET status = 'rejected', reviewed_at = ?, reviewed_by = ? WHERE id = ?", time.Now().Unix(), authenticatedUser, requestID)
			}

			conn.WriteJSON(map[string]string{"type": "friend_request_reviewed", "action": action, "request_id": strconv.Itoa(requestID)})
			sendSyncData(fromUser)
			sendSyncData(toUser)

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

	friendRequestsRows, err := db.Query(`SELECT id, from_user, to_user, message, status, created_at, reviewed_at, reviewed_by FROM friend_requests WHERE from_user = ? OR to_user = ? ORDER BY created_at DESC`, username, username)
	friendRequests := make([]map[string]interface{}, 0)
	if err == nil {
		for friendRequestsRows.Next() {
			var id int
			var fromUser, toUser, message, status, reviewedBy string
			var createdAt, reviewedAt int64
			_ = friendRequestsRows.Scan(&id, &fromUser, &toUser, &message, &status, &createdAt, &reviewedAt, &reviewedBy)
			friendRequests = append(friendRequests, map[string]interface{}{
				"id":          id,
				"from_user":   fromUser,
				"to_user":     toUser,
				"message":     message,
				"status":      status,
				"created_at":  createdAt,
				"reviewed_at": reviewedAt,
				"reviewed_by": reviewedBy,
			})
		}
		friendRequestsRows.Close()
	}

	_ = conn.WriteJSON(map[string]interface{}{
		"type":            "sync_data",
		"friends":         friends,
		"groups":          syncGroups,
		"friend_requests": friendRequests,
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
	hasConfig := false
	if p, ok := body["provider"].(string); ok {
		cfg["provider"] = p
		if strings.TrimSpace(p) != "" {
			hasConfig = true
		}
	}
	if k, ok := body["keys"].(map[string]interface{}); ok {
		cfg["keys"] = k
		for _, v := range k {
			if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
				hasConfig = true
				break
			}
		}
	}
	if err := setAIConfigInDB(cfg); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if hasConfig {
		_ = setExtensionInDB("ai_chat", true)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// 管理接口：设置/获取 secure_ws key（仅管理员）
func handleAdminSecureWS(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if !checkAdminSecret(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		key, err := getSecureWSKeyFromDB()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"key": key})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if !checkAdminSecret(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var body map[string]string
	if err := json.Unmarshal(raw, &body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	k := body["key"]
	if err := setSecureWSKeyInDB(k); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// 刷新 token（管理员或用户可调用以续期自己的 token）
func handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(raw))
	var body map[string]string
	if err := json.Unmarshal(raw, &body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	old := body["token"]
	if old == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	user, err := getUsernameByToken(old)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	newToken := generateToken()
	if err := saveSessionToken(user, newToken); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": newToken})
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
	// preserve body because checkAdminSecret may read it
	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if !checkAdminSecret(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
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
	// request payload parsed below into generic map
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
	// 允许请求体中临时指定 provider（覆盖后端配置），例如用于前端直接指定测试 provider
	provider := ""
	if p, ok := payload["provider"].(string); ok && p != "" {
		provider = p
	}
	// 读取 AI 配置并根据 provider 调用对应实现
	cfg, _ := getAIConfigFromDB()
	if provider == "" {
		if pv, ok := cfg["provider"].(string); ok {
			provider = pv
		}
	}
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
func callProviderURL(provider string, keys map[string]string) (string, error) {
	modelKey := provider + "_model"
	urlKey := provider + "_url"
	apiKey := keys[provider+"_key"]
	model := keys[modelKey]
	apiURL := keys[urlKey]
	if apiKey == "" {
		return "", fmt.Errorf("%s 未配置 (需要 %s_key)", strings.Title(provider), provider)
	}
	if apiURL == "" {
		apiURL = deriveDefaultProviderURL(provider, model)
	}
	if apiURL == "" {
		if model != "" {
			return "", fmt.Errorf("%s 未配置 (无法从模型 %s 推断 URL)", strings.Title(provider), model)
		}
		return "", fmt.Errorf("%s 未配置 (需要 %s_url 或 %s_model)", strings.Title(provider), provider, provider)
	}
	return apiURL, nil
}

func deriveDefaultProviderURL(provider, model string) string {
	provider = strings.ToLower(provider)
	model = strings.TrimSpace(model)
	switch provider {
	case "deepseek":
		if model == "" {
			return "https://api.deepseek.example/v1/chat"
		}
		return fmt.Sprintf("https://api.deepseek.example/v1/models/%s/chat", model)
	case "gemini":
		if model == "" {
			return "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"
		}
		return fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	case "claude":
		return "https://api.anthropic.com/v1/complete"
	}
	return ""
}

func callDeepseek(prompt string, keys map[string]string) (string, error) {
	apiURL, err := callProviderURL("deepseek", keys)
	if err != nil {
		return "", err
	}
	headers := map[string]string{}
	for k, v := range keys {
		if strings.HasPrefix(k, "deepseek_header_") {
			h := strings.TrimPrefix(k, "deepseek_header_")
			headers[h] = v
		}
	}
	return callProviderHTTP(apiURL, keys["deepseek_key"], "deepseek", map[string]interface{}{"query": prompt}, headers)
}

func callGemini(prompt string, keys map[string]string) (string, error) {
	apiURL, err := callProviderURL("gemini", keys)
	if err != nil {
		return "", err
	}
	headers := map[string]string{}
	for k, v := range keys {
		if strings.HasPrefix(k, "gemini_header_") {
			h := strings.TrimPrefix(k, "gemini_header_")
			headers[h] = v
		}
	}
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{{
			"role":  "user",
			"parts": []map[string]interface{}{{"text": prompt}},
		}},
	}
	return callProviderHTTP(apiURL, keys["gemini_key"], "gemini", payload, headers)
}

func callClaude(prompt string, keys map[string]string) (string, error) {
	apiURL, err := callProviderURL("claude", keys)
	if err != nil {
		return "", err
	}
	model := keys["claude_model"]
	if model == "" {
		model = "claude-2"
	}
	headers := map[string]string{}
	for k, v := range keys {
		if strings.HasPrefix(k, "claude_header_") {
			h := strings.TrimPrefix(k, "claude_header_")
			headers[h] = v
		}
	}
	return callProviderHTTP(apiURL, keys["claude_key"], "claude", map[string]interface{}{"model": model, "prompt": prompt}, headers)
}

// 通用 HTTP 调用提供者
func callProviderHTTP(apiURL, apiKey, provider string, payload interface{}, extraHeaders map[string]string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	// 常见的鉴权头：尝试使用 Bearer 或 X-API-Key / x-api-key
	if provider == "claude" {
		req.Header.Set("x-api-key", apiKey)
	} else if provider == "gemini" {
		req.Header.Set("x-goog-api-key", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	// 额外自定义头
	for hk, hv := range extraHeaders {
		if hk == "" || hv == "" {
			continue
		}
		req.Header.Set(hk, hv)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s", friendlyProviderError(resp.StatusCode, respBody, provider))
	}
	// 尝试解析常见字段
	var jr map[string]interface{}
	_ = json.Unmarshal(respBody, &jr)
	// 常见返回解析规则： reply/text/completion/result/output/candidates/outputs
	if v, ok := jr["reply"].(string); ok && v != "" {
		return v, nil
	}
	if v, ok := jr["text"].(string); ok && v != "" {
		return v, nil
	}
	if v, ok := jr["completion"].(string); ok && v != "" {
		return v, nil
	}
	if v, ok := jr["result"].(map[string]interface{}); ok {
		if s, ok2 := v["output"].(string); ok2 && s != "" {
			return s, nil
		}
	}
	// Gemini-style: candidates -> content.text / parts.text
	if cands, ok := jr["candidates"].([]interface{}); ok && len(cands) > 0 {
		if first, ok := cands[0].(map[string]interface{}); ok {
			if cont, ok2 := first["content"].(map[string]interface{}); ok2 {
				if txt, ok3 := cont["text"].(string); ok3 && txt != "" {
					return txt, nil
				}
				if parts, ok4 := cont["parts"].([]interface{}); ok4 && len(parts) > 0 {
					for _, part := range parts {
						if pmap, ok5 := part.(map[string]interface{}); ok5 {
							if txt2, ok6 := pmap["text"].(string); ok6 && txt2 != "" {
								return txt2, nil
							}
						}
					}
				}
			}
			if txt, ok4 := first["text"].(string); ok4 && txt != "" {
				return txt, nil
			}
		}
	}
	// outputs array with text
	if outs, ok := jr["outputs"].([]interface{}); ok && len(outs) > 0 {
		if o0, ok := outs[0].(map[string]interface{}); ok {
			if txt, ok2 := o0["text"].(string); ok2 && txt != "" {
				return txt, nil
			}
		}
	}
	// 兜底返回原始 body 文本
	return string(respBody), nil
}

func friendlyProviderError(statusCode int, body []byte, provider string) string {
	providerName := strings.TrimSpace(provider)
	if providerName == "" {
		providerName = "AI"
	}

	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &parsed)

	msg := strings.TrimSpace(parsed.Error.Message)
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	msg = strings.ReplaceAll(msg, "\n", " ")

	switch {
	case statusCode == http.StatusTooManyRequests || strings.Contains(strings.ToLower(msg), "quota") || strings.Contains(strings.ToLower(msg), "resource_exhausted"):
		return fmt.Sprintf("%s 服务配额已用尽，请稍后再试，或更换为可用的 API Key / 模型。", providerName)
	case statusCode == http.StatusUnauthorized || strings.Contains(strings.ToLower(msg), "unauthorized") || strings.Contains(strings.ToLower(msg), "api key"):
		return fmt.Sprintf("%s 服务鉴权失败，请检查 API Key 是否正确。", providerName)
	case statusCode == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "not found"):
		return fmt.Sprintf("%s 服务模型或接口不存在，请检查模型名称和接口配置。", providerName)
	default:
		if msg != "" {
			return fmt.Sprintf("%s 服务返回错误：%s", providerName, msg)
		}
		return fmt.Sprintf("%s 服务返回错误（%d）。", providerName, statusCode)
	}
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
	// 支持字段名 file 或 image 上传
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		// ignore parse error, try to continue
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("image")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "无效的文件"})
			return
		}
	}
	defer file.Close()

	visibility := strings.ToLower(strings.TrimSpace(r.FormValue("visibility"))) // public or private
	username := strings.TrimSpace(r.FormValue("username"))
	if username == "" {
		username = "anonymous"
	}

	// 存放到 photos/uploads 下：公开 -> userpublic，私密 -> photos/uploads/<username>
	baseDir := "./photos/uploads"
	var uploadDir string
	if visibility == "public" {
		uploadDir = filepath.Join(baseDir, "userpublic")
	} else {
		uploadDir = filepath.Join(baseDir, username)
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "无法创建目录"})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	newFileName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(randBytes), ext)
	savePath := filepath.Join(uploadDir, newFileName)

	out, err := os.Create(savePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "无法保存文件"})
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "写入文件失败"})
		return
	}

	// 返回可访问的 URL
	var url string
	if visibility == "public" {
		url = "/photos/uploads/userpublic/" + newFileName
	} else {
		url = "/photos/uploads/" + username + "/" + newFileName
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"url": url})
}

// 受保护的静态文件访问：公开目录 (/photos/uploads/userpublic) 直接返回，
// 私有目录 (/photos/uploads/<username>/...) 需要 Authorization: Bearer <token> 验证
func handleProtectedUploads(w http.ResponseWriter, r *http.Request) {
	// 请求的路径以 /photos/uploads/ 开头，由于路由是注册在该前缀下，获取相对路径
	rel := strings.TrimPrefix(r.URL.Path, "/photos/uploads/")
	// 如果请求以 userpublic/ 开头，直接返回静态文件
	if strings.HasPrefix(rel, "userpublic/") {
		filePath := filepath.Join("./photos/uploads", rel)
		http.ServeFile(w, r, filePath)
		return
	}

	// 私有资源：第一段应该是用户名
	parts := strings.SplitN(rel, "/", 2)
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	username := parts[0]
	// 通过 token 验证请求者是否为同一用户或管理员
	token := getTokenFromHeader(r)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	reqUser, err := getUsernameByToken(token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if reqUser != username {
		// 非资源拥有者，拒绝访问
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// 构造实际文件路径并返回
	filePath := filepath.Join("./photos/uploads", rel)
	http.ServeFile(w, r, filePath)
}

// 列出可用的用户或公共资产（背景图 / 字体）
func handleAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}
	scope := r.URL.Query().Get("scope") // public or user
	username := r.URL.Query().Get("username")
	fileType := strings.ToLower(r.URL.Query().Get("type")) // optional: background|font

	base := "./photos/uploads"
	var dir string
	var urlPrefix string
	if scope == "user" {
		if username == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "缺少用户名"})
			return
		}
		dir = filepath.Join(base, username)
		urlPrefix = "/photos/uploads/" + username + "/"
	} else {
		// 默认或 public
		dir = filepath.Join(base, "userpublic")
		urlPrefix = "/photos/uploads/userpublic/"
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		// 如果目录不存在，则返回空数组
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"assets": []string{}})
		return
	}

	allowed := map[string]bool{}
	if fileType == "font" {
		for _, e := range []string{".ttf", ".otf", ".woff", ".woff2"} {
			allowed[e] = true
		}
	} else if fileType == "background" {
		for _, e := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
			allowed[e] = true
		}
	}

	var assets []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if len(allowed) > 0 {
			if !allowed[strings.ToLower(filepath.Ext(name))] {
				continue
			}
		}
		assets = append(assets, urlPrefix+name)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"assets": assets})
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
	if !checkAdminSecret(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req map[string]string
	_ = json.NewDecoder(r.Body).Decode(&req)

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
	if !checkAdminSecret(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&req)

	idFloat, _ := req["id"].(float64)
	_, _ = db.Exec("DELETE FROM messages WHERE id = ?", int64(idFloat))
	w.WriteHeader(http.StatusOK)
}

// 管理员登录（返回 token）
func handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 支持两种登录：数据库中的 admin 用户 或 使用预设的 adminSecret
	if req.Username != "admin" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	// 优先尝试数据库验证，如果不存在或不匹配则回退到配置文件中的 adminpassword
	var dbPwd string
	err := db.QueryRow("SELECT password FROM users WHERE username = ?", req.Username).Scan(&dbPwd)
	if err == nil && strings.EqualFold(dbPwd, req.Password) {
		// 通过数据库认证
		fmt.Printf("[INFO] admin login: db auth success for user=%s\n", req.Username)
	} else if strings.EqualFold(req.Password, getAdminPassword()) {
		// 通过配置文件中的 secret 认证
		fmt.Printf("[INFO] admin login: fallback secret auth for user=%s\n", req.Username)
	} else {
		fmt.Printf("[WARN] admin login: auth failed for user=%s\n", req.Username)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	token := generateToken()
	_ = saveSessionToken(req.Username, token)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
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
	if !checkAdminSecret(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	var req map[string]string
	_ = json.NewDecoder(r.Body).Decode(&req)

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

	var avatar, signature, background, font string
	err := db.QueryRow("SELECT avatar_url, signature, background_url, font_url FROM users WHERE username = ?", username).Scan(&avatar, &signature, &background, &font)
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
		"font_url":       font,
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

	// 支持两种用法：
	// 1) 以 multipart/form-data 上传文件（保持兼容旧用法）
	// 2) 以 application/json 传入 { "url": "..." } 来设置已上传的文件 URL
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var payload struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload.URL == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = db.Exec("UPDATE users SET background_url = ? WHERE username = ?", payload.URL, user)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"url": payload.URL})
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

func handleUserFont(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	token := getTokenFromHeader(r)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	user, err := getUsernameByToken(token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var payload struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload.URL == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = db.Exec("UPDATE users SET font_url = ? WHERE username = ?", payload.URL, user)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"url": payload.URL})
		return
	}

	// 不支持直接上传字体文件到此接口；请先使用 /api/upload 上传，然后用 JSON 调用此接口设置 URL
	w.WriteHeader(http.StatusBadRequest)
}
