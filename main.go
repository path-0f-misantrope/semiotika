package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"poznovatel/database"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	dsn := "postgres://postgres:1234@localhost:8080/postgres"
	dbConnect, err := pgx.Connect(context.Background(), dsn)
	database.DbInit(dbConnect)
	r.Use(func(c *gin.Context) {
		c.Set("db", dbConnect)
		c.Next()
	})
	if err != nil {
		fmt.Printf("дб упала, %s", err)
	}


	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})
	r.GET("/register", showRegisterPage)
	r.POST("/register", register)
	r.POST("/upload", mainwork)
	r.GET("/dashboard", requireAuth, dashboard)
	r.GET("/login", showLoginPage)
	r.POST("/login", login)
	r.POST("/upload-auth", authvedmainwork)
	r.Run(":8787")
}

func getAbsPath(rel string) string {
	abs, _ := os.Getwd()
	return fmt.Sprintf("%s/%s", abs, rel)
}
func mainwork(c *gin.Context) {
	fmt.Println("Upload handler triggered!")
	file, err := c.FormFile("audio")
	if err != nil {
		c.String(http.StatusBadRequest, "Ошибка загрузки файла: %v", err)
		return
	}

	filename := file.Filename
	savePath := fmt.Sprintf("audio/%s", filename) // сохраняем в папку audio
	c.SaveUploadedFile(file, savePath)

	// 1. Запускаем Demucs
	cmdDemucs := exec.Command("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/data", getAbsPath("audio")), // монтируем папку
		"my-demucs",
		"--two-stems", "vocals",
		filename)

	if err := cmdDemucs.Run(); err != nil {
		return
	}
	fmt.Print("демукс отработал штатно")
	nameOnly := strings.TrimSuffix(filename, filepath.Ext(filename))

	// получаем абсолютный путь до vocals.wav
	absVocalsPath := filepath.Join("D:/poznovatel/audio/separated/htdemucs", nameOnly, "vocals.wav")

	// проверка, существует ли файл
	if _, err := os.Stat(absVocalsPath); os.IsNotExist(err) {
		c.String(http.StatusInternalServerError, "Файл vocals.wav не найден по пути %s", absVocalsPath)
		return
	}
	// 2 обращение к висперу
	client := &http.Client{}
	jsonData := map[string]string{
		"filename": absVocalsPath,
	}
	jsonValue, _ := json.Marshal(jsonData)
	resp, err := client.Post("http://localhost:5000/whhisper", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		panic(fmt.Errorf("ошибка запроса к whisper-сервису: %v", err))
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		panic(fmt.Errorf("не удалось декодировать JSON: %v", err))
	}

	c.JSON(http.StatusOK, result)
}

func register(c *gin.Context) {
	db := c.MustGet("db").(*pgx.Conn)
	username := c.PostForm("username")
	password := c.PostForm("password")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.String(http.StatusInternalServerError, "Ошибка хеширования пароля")
		return
	}

	_, err = db.Exec(context.Background(), "INSERT INTO users (username, password_hash) VALUES ($1, $2)", username, hashedPassword)
	if err != nil {
		c.String(http.StatusInternalServerError, "Ошибка при сохранении пользователя")
		return
	}

	c.Redirect(http.StatusFound, "/login")

}
func login(c *gin.Context) {
	db := c.MustGet("db").(*pgx.Conn)
	username := c.PostForm("username")
	password := c.PostForm("password")
	var user_id int
	var hashedPassword string
	err := db.QueryRow(context.Background(), "SELECT id,password_hash FROM users WHERE username=$1", username).Scan(&user_id, &hashedPassword)
	if err != nil {
		c.String(http.StatusUnauthorized, "Пользователь не найден")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		c.String(http.StatusUnauthorized, "Неверный пароль")
		return
	}

	// Установка куки
	c.SetCookie("user", username, 3600, "/", "localhost", false, true)
	c.SetCookie("user_id", fmt.Sprintf("%d", user_id), 3600, "/", "localhost", false, true)
	c.Redirect(http.StatusFound, "/dashboard")
}

func requireAuth(c *gin.Context) {
	username, err := c.Cookie("user")
	if err != nil || username == "" {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}
	c.Next()
}

func dashboard(c *gin.Context) {
	db := c.MustGet("db").(*pgx.Conn)

	userIDStr, err1 := c.Cookie("user_id")
	username, err2 := c.Cookie("user")

	if err1 != nil || err2 != nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	// Преобразуем userID из строки в int
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	// Загружаем песни из БД
	rows, err := db.Query(context.Background(), `SELECT title, lyrics FROM songs WHERE user_id = $1`, userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Ошибка загрузки песен")
		return
	}
	defer rows.Close()

	var songs []map[string]string
	for rows.Next() {
		var title, lyrics string
		if err := rows.Scan(&title, &lyrics); err == nil {
			songs = append(songs, map[string]string{
				"Title":  title,
				"Lyrics": lyrics,
			})
		}
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Username": username,
		"Songs":    songs,
	})
}
func showLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}
func showRegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", nil)
}

func authvedmainwork(c *gin.Context) {
	fmt.Println(" auth Upload handler triggered!")
	db := c.MustGet("db").(*pgx.Conn)
	userIDStr, err := c.Cookie("user_id")
	if err != nil {
		c.Redirect(http.StatusForbidden, "/login")
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	file, err := c.FormFile("audio")
	if err != nil {
		c.String(http.StatusBadRequest, "Ошибка загрузки файла: %v", err)
		return
	}

	filename := file.Filename
	savePath := fmt.Sprintf("audio/%s", filename)
	c.SaveUploadedFile(file, savePath)

	// 1. Запускаем Demucs
	cmdDemucs := exec.Command("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/data", getAbsPath("audio")),
		"my-demucs",
		"--two-stems", "vocals",
		filename) 

	if err := cmdDemucs.Run(); err != nil {
		return
	}
	fmt.Print("демукс отработал штатно")
	// получаем имя файла без расширения,
	nameOnly := strings.TrimSuffix(filename, filepath.Ext(filename))

	// получаем абсолютный путь до vocals.wav
	absVocalsPath := filepath.Join("D:/poznovatel/audio/separated/htdemucs", nameOnly, "vocals.wav")
	fmt.Println("Отправляем путь:", absVocalsPath)

	// проверка, существует ли файл
	if _, err := os.Stat(absVocalsPath); os.IsNotExist(err) {
		c.String(http.StatusInternalServerError, "Файл vocals.wav не найден по пути %s", absVocalsPath)
		return
	}
	// 2 обращение к висперу
	client := &http.Client{}
	jsonData := map[string]string{
		"filename": absVocalsPath}
	jsonValue, _ := json.Marshal(jsonData)
	resp, err := client.Post("http://localhost:5000/whhisper", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		panic(fmt.Errorf("ошибка запроса к whisper-сервису: %v", err))
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		panic(fmt.Errorf("не удалось декодировать JSON: %v", err))
	}

	lines, ok := result["transcription"].([]interface{})
	if !ok {
		c.String(http.StatusInternalServerError, "Ошибка: текст не найден в результате Whisper")
		return
	}

	var lyrics string
	for _, line := range lines {
		strLine, ok := line.(string)
		if ok {
			lyrics += strLine + "\n"
		}
	}
	_, err = db.Exec(context.Background(), "INSERT INTO songs (user_id, title, lyrics) VALUES ($1, $2, $3)", userID, filename, lyrics)
	if err != nil {
		fmt.Println("ошибка сохранения в бд ")
	}
	os.Remove(savePath)
	os.RemoveAll(filepath.Join("D:/poznovatel/audio/separated/htdemucs", nameOnly))
	c.Redirect(http.StatusSeeOther, "/dashboard")

}

