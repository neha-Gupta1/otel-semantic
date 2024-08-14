package models

var UsersCol = "users"

type User struct {
	ID      string `json:"id" binding:"required"`
	Name    string `json:"name" binding:"required"`
	PhoneNo int    `json:"phone_no" binding:"required"`
}
