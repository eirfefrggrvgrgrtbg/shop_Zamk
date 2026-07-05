package main

import (
	"context"
	"fmt"
	"os"
	
	"golang.org/x/crypto/bcrypt"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://zamk:zamk_password@localhost:5433/zamk?sslmode=disable")
	if err != nil {
		fmt.Println("Error connecting to db:", err)
		os.Exit(1)
	}
	defer pool.Close()

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	_, err = pool.Exec(context.Background(), "UPDATE users SET password_hash = $1 WHERE email = 'admin@zamk.local'", string(hash))
	if err != nil {
		fmt.Println("Error querying db:", err)
		os.Exit(1)
	}

	fmt.Println("Admin password updated!")
}
