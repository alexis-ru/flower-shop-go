package main

import (
	"bufio"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

//go:embed web
var webFS embed.FS

var config Config

// ---------- Config ----------

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	ServerHost string
	ServerPort string
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	file, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		switch section {
		case "database":
			switch k {
			case "host":
				cfg.DBHost = v
			case "port":
				cfg.DBPort = v
			case "user":
				cfg.DBUser = v
			case "password":
				cfg.DBPassword = v
			case "dbname":
				cfg.DBName = v
			}
		case "server":
			switch k {
			case "host":
				cfg.ServerHost = v
			case "port":
				cfg.ServerPort = v
			}
		}
	}
	return cfg, scanner.Err()
}

func (c Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}

// ---------- Session ----------

type Session struct {
	UserID int
	Role   string
	Login  string
	Name   string
}

var (
	sessions = make(map[string]*Session)
	mu       sync.Mutex
)

func createSession(sid string, s *Session) {
	mu.Lock()
	sessions[sid] = s
	mu.Unlock()
}

func getSession(r *http.Request) *Session {
	c, err := r.Cookie("session_id")
	if err != nil {
		return nil
	}
	mu.Lock()
	s := sessions[c.Value]
	mu.Unlock()
	return s
}

func deleteSession(r *http.Request) {
	c, err := r.Cookie("session_id")
	if err != nil {
		return
	}
	mu.Lock()
	delete(sessions, c.Value)
	mu.Unlock()
}

// ---------- Models ----------

type User struct {
	ID               int        `json:"id"`
	FullName         string     `json:"full_name"`
	Login            string     `json:"login"`
	Role             string     `json:"role"`
	Status           string     `json:"status"`
	RegistrationDate time.Time  `json:"registration_date"`
	DismissalDate    *time.Time `json:"dismissal_date"`
	BlockDate        *time.Time `json:"block_date"`
}

type Flower struct {
	ID          int     `json:"id"`
	SellerID    int     `json:"seller_id"`
	SellerName  string  `json:"seller_name"`
	Name        string  `json:"name"`
	Quantity    int     `json:"quantity"`
	ArrivalDate *string `json:"arrival_date"`
	SaleDate    *string `json:"sale_date"`
}

// ---------- DB ----------

var db *sql.DB

func seedDirector() {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE role='director')").Scan(&exists)
	if err != nil || exists {
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	db.Exec(`INSERT INTO users (full_name, login, password_hash, role, status)
		VALUES ($1, $2, $3, 'director', 'active')`,
		"Директор", "director", string(hash))
	log.Println("Initial director account created: director / admin123")
}

// ---------- Handlers: Auth ----------

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Неверный запрос", http.StatusBadRequest)
		return
	}

	var u User
	var hash string
	err := db.QueryRow(`SELECT id, full_name, login, password_hash, role, status FROM users
		WHERE login=$1`, body.Login).Scan(&u.ID, &u.FullName, &u.Login, &hash, &u.Role, &u.Status)
	if err != nil {
		jsonError(w, "Неверный логин или пароль", http.StatusUnauthorized)
		return
	}
	if u.Status != "active" {
		jsonError(w, "Учётная запись заблокирована или сотрудник уволен", http.StatusForbidden)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		jsonError(w, "Неверный логин или пароль", http.StatusUnauthorized)
		return
	}

	sid := generateToken()
	createSession(sid, &Session{UserID: u.ID, Role: u.Role, Login: u.Login, Name: u.FullName})
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})
	jsonResp(w, map[string]any{"role": u.Role, "name": u.FullName})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	deleteSession(r)
	http.SetCookie(w, &http.Cookie{Name: "session_id", Path: "/", MaxAge: -1})
	jsonResp(w, map[string]string{"status": "ok"})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil {
		jsonResp(w, map[string]any{"authenticated": false})
		return
	}
	jsonResp(w, map[string]any{
		"authenticated": true,
		"role":          s.Role,
		"name":           s.Name,
		"user_id":        s.UserID,
	})
}

// ---------- Handlers: Director (Users) ----------

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil || s.Role != "director" {
		jsonError(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	rows, err := db.Query(`SELECT id, full_name, login, role, status,
		registration_date, dismissal_date, block_date FROM users ORDER BY id`)
	if err != nil {
		jsonError(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.FullName, &u.Login, &u.Role, &u.Status,
			&u.RegistrationDate, &u.DismissalDate, &u.BlockDate); err != nil {
			continue
		}
		users = append(users, u)
	}
	jsonResp(w, users)
}

func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil || s.Role != "director" {
		jsonError(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	var body struct {
		FullName string `json:"full_name"`
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Неверный запрос", http.StatusBadRequest)
		return
	}
	if body.FullName == "" || body.Login == "" || body.Password == "" {
		jsonError(w, "Заполните все поля", http.StatusBadRequest)
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	_, err := db.Exec(`INSERT INTO users (full_name, login, password_hash, role, status)
		VALUES ($1, $2, $3, 'seller', 'active')`,
		body.FullName, body.Login, string(hash))
	if err != nil {
		jsonError(w, "Логин уже занят", http.StatusConflict)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

func handleUserAction(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil || s.Role != "director" {
		jsonError(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonError(w, "Неверный URL", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(parts[3])
	if err != nil {
		jsonError(w, "Неверный ID", http.StatusBadRequest)
		return
	}
	action := parts[4]

	switch action {
	case "block":
		_, err = db.Exec("UPDATE users SET status='blocked', block_date=NOW() WHERE id=$1 AND role='seller'", id)
	case "unblock":
		_, err = db.Exec("UPDATE users SET status='active', block_date=NULL WHERE id=$1", id)
	case "fire":
		_, err = db.Exec("UPDATE users SET status='fired', dismissal_date=NOW() WHERE id=$1 AND role='seller'", id)
	case "delete":
		if id == s.UserID {
			jsonError(w, "Нельзя удалить самого себя", http.StatusBadRequest)
			return
		}
		_, err = db.Exec("DELETE FROM users WHERE id=$1 AND role='seller'", id)
	default:
		jsonError(w, "Неизвестное действие", http.StatusBadRequest)
		return
	}
	if err != nil {
		jsonError(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

// ---------- Handlers: Flowers ----------

func handleListFlowers(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil {
		jsonError(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	var rows *sql.Rows
	var err error
	if s.Role == "director" {
		rows, err = db.Query(`SELECT f.id, f.seller_id, u.full_name, f.name, f.quantity,
			f.arrival_date::text, f.sale_date::text
			FROM flowers f JOIN users u ON f.seller_id=u.id ORDER BY f.id DESC`)
	} else {
		rows, err = db.Query(`SELECT f.id, f.seller_id, u.full_name, f.name, f.quantity,
			f.arrival_date::text, f.sale_date::text
			FROM flowers f JOIN users u ON f.seller_id=u.id
			WHERE f.seller_id=$1 ORDER BY f.id DESC`, s.UserID)
	}
	if err != nil {
		jsonError(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var flowers []Flower
	for rows.Next() {
		var f Flower
		if err := rows.Scan(&f.ID, &f.SellerID, &f.SellerName, &f.Name, &f.Quantity,
			&f.ArrivalDate, &f.SaleDate); err != nil {
			continue
		}
		flowers = append(flowers, f)
	}
	if flowers == nil {
		flowers = []Flower{}
	}
	jsonResp(w, flowers)
}

func handleCreateFlower(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil || s.Role != "seller" {
		jsonError(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	var body struct {
		Name        string  `json:"name"`
		Quantity    int     `json:"quantity"`
		ArrivalDate *string `json:"arrival_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		jsonError(w, "Заполните наименование", http.StatusBadRequest)
		return
	}
	var arrival sql.NullString
	if body.ArrivalDate != nil && *body.ArrivalDate != "" {
		arrival = sql.NullString{String: *body.ArrivalDate, Valid: true}
	}
	_, err := db.Exec(`INSERT INTO flowers (seller_id, name, quantity, arrival_date)
		VALUES ($1, $2, $3, $4)`,
		s.UserID, body.Name, body.Quantity, arrival)
	if err != nil {
		jsonError(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

func handleFlowerAction(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s == nil || s.Role != "seller" {
		jsonError(w, "Доступ запрещён", http.StatusForbidden)
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		jsonError(w, "Неверный URL", http.StatusBadRequest)
		return
	}
	id, _ := strconv.Atoi(parts[3])
	action := parts[4]

	switch action {
	case "sale":
		var body struct {
			SaleDate *string `json:"sale_date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Неверный запрос", http.StatusBadRequest)
			return
		}
		if body.SaleDate != nil && *body.SaleDate != "" {
			db.Exec("UPDATE flowers SET sale_date=$1 WHERE id=$2 AND seller_id=$3",
				*body.SaleDate, id, s.UserID)
		}
	case "delete":
		db.Exec("DELETE FROM flowers WHERE id=$1 AND seller_id=$2", id, s.UserID)
	default:
		jsonError(w, "Неизвестное действие", http.StatusBadRequest)
		return
	}
	jsonResp(w, map[string]string{"status": "ok"})
}

// ---------- Helpers ----------

func generateToken() string {
	return fmt.Sprintf("%d-%x", time.Now().UnixNano(), time.Now().Unix())
}

func jsonResp(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ---------- Main ----------

func main() {
	cfgPath := "config.ini"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	var err error
	config, err = loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Не удалось прочитать %s: %v", cfgPath, err)
	}

	db, err = sql.Open("postgres", config.DSN())
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("Ожидание БД... (%d/10)", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("БД недоступна: %v", err)
	}

	seedDirector()
	log.Println("БД подключена, директор создан при необходимости")

	mux := http.NewServeMux()

	// API
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/logout", handleLogout)
	mux.HandleFunc("/api/me", handleMe)
	mux.HandleFunc("/api/users", handleListUsers)
	mux.HandleFunc("/api/users/create", handleCreateUser)
	mux.HandleFunc("/api/users/", handleUserAction)
	mux.HandleFunc("/api/flowers", handleListFlowers)
	mux.HandleFunc("/api/flowers/create", handleCreateFlower)
	mux.HandleFunc("/api/flowers/", handleFlowerAction)

	// Static files (embedded)
	webSub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(webSub)))

	addr := config.ServerHost + ":" + config.ServerPort
	log.Printf("Сервер «Василёк» запущен на http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
