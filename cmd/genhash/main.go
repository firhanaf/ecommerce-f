package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "Admin@12345"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Password : %s\n", password)
	fmt.Printf("Hash     : %s\n", hash)
}
