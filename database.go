package database

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

func DbInit(connect *pgx.Conn) {
	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	createSongsTable := `
	CREATE TABLE IF NOT EXISTS songs (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		lyrics TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := connect.Exec(context.Background(), createUsersTable)
	if err != nil {
		log.Fatal("Ошибка при создании таблицы users:", err)
	}
	fmt.Println("Таблица users создана")

	_, err = connect.Exec(context.Background(), createSongsTable)
	if err != nil {
		log.Fatal("Ошибка при создании таблицы songs:", err)
	}
	fmt.Println("Таблица songs создана")

}
